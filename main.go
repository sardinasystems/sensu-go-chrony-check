package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/facebookincubator/ntp/protocol/chrony"
	"github.com/sensu-community/sensu-plugin-sdk/sensu"
	"github.com/sensu/sensu-go/types"
)

// Config represents the check plugin config.
type Config struct {
	sensu.PluginConfig
	SocketPath string
	// Jitter?
	StratumWarning  uint
	StratumCritical uint
	//OffsetWarning  float64
	//OffsetCritical float64
	LastRxWarning        uint
	LastRxCritical       uint
	ReachabilityWarning  float64
	ReachabilityCritical float64
	Debug                bool
}

var (
	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "sensu-go-chrony-check",
			Short:    "sensu-go check of chrony status",
			Keyspace: "sensu.io/plugins/sensu-go-chrony-check/config",
		},
	}

	options = []*sensu.PluginConfigOption{
		{
			Path:      "socket",
			Env:       "HAPROXY_SOCKET",
			Argument:  "socket",
			Shorthand: "S",
			Default:   chrony.ChronySocketPath,
			Usage:     "Path to haproxy control socket",
			Value:     &plugin.SocketPath,
		},
		{
			Path:     "stratum_warning",
			Env:      "CHRONY_STRATUM_WARNING",
			Argument: "stratum-warning",
			//Shorthand: "sw",
			Default: uint(10),
			Usage:   "Stratum warning level",
			Value:   &plugin.StratumWarning,
		},
		{
			Path:     "stratum_critical",
			Env:      "CHRONY_STRATUM_CRITICAL",
			Argument: "stratum-critical",
			//Shorthand: "sc",
			Default: uint(12),
			Usage:   "Stratum critical level",
			Value:   &plugin.StratumCritical,
		},
		// {
		// 	Path:      "offset_warning",
		// 	Env:       "CHRONY_OFFSET_WARNING",
		// 	Argument:  "offset-warning",
		// 	Shorthand: "ow",
		// 	Default:   0.050,
		// 	Usage:     "Offset warning level [s]",
		// 	Value:     &plugin.OffsetWarning,
		// },
		// {
		// 	Path:      "offset_critical",
		// 	Env:       "CHRONY_OFFSET_CRITICAL",
		// 	Argument:  "offset-critical",
		// 	Shorthand: "oc",
		// 	Default:   0.100,
		// 	Usage:     "Offset critical level [s]",
		// 	Value:     &plugin.OffsetCritical,
		// },
		{
			Path:      "lastrx_warning",
			Env:       "CHRONY_LASTRX_WARNING",
			Argument:  "lastrx-warning",
			Shorthand: "W",
			Default:   uint(64),
			Usage:     "LastRx warning level [s]",
			Value:     &plugin.LastRxWarning,
		},
		{
			Path:      "lastrx_critical",
			Env:       "CHRONY_LASTRX_CRITICAL",
			Argument:  "lastrx-critical",
			Shorthand: "C",
			Default:   uint(128),
			Usage:     "LastRx critical level [s]",
			Value:     &plugin.LastRxCritical,
		},
		{
			Path:      "reachability_warning",
			Env:       "CHRONY_REACHABILITY_WARNING",
			Argument:  "reachablility-warning",
			Shorthand: "w",
			Default:   67.0,
			Usage:     "Reachablility warning percent",
			Value:     &plugin.ReachabilityWarning,
		},
		{
			Path:      "reachability_critical",
			Env:       "CHRONY_REACHABILITY_CRITICAL",
			Argument:  "reachablility-critical",
			Shorthand: "c",
			Default:   34.0,
			Usage:     "Reachablility critical percent",
			Value:     &plugin.ReachabilityCritical,
		},
		{
			Path:      "debug",
			Env:       "CHRONY_DEBUG",
			Argument:  "debug",
			Shorthand: "d",
			Default:   false,
			Usage:     "output debugging data",
			Value:     &plugin.Debug,
		},
	}
)

func main() {
	check := sensu.NewGoCheck(&plugin.PluginConfig, options, checkArgs, executeCheck, false)
	check.Execute()
}

func checkArgs(event *types.Event) (int, error) {
	path, err := filepath.Abs(plugin.SocketPath)
	if err != nil {
		return sensu.CheckStateUnknown, fmt.Errorf("--socket error: %w", err)
	}

	fi, err := os.Lstat(path)
	if err != nil {
		return sensu.CheckStateUnknown, fmt.Errorf("--socket error: %w", err)
	} else if fi.Mode()&os.ModeSocket == 0 {
		return sensu.CheckStateUnknown, fmt.Errorf("--socket: %s is not socket: %v", path, fi.Mode())
	}
	plugin.SocketPath = path

	return sensu.CheckStateOK, nil
}

func executeCheck(event *types.Event) (int, error) {
	stats, err := getStats(plugin.SocketPath)
	if err != nil {
		return sensu.CheckStateUnknown, err
	}

	// Print debug data on exit
	defer func() {
		if plugin.Debug {
			b, _ := json.MarshalIndent(stats, "", "  ")
			fmt.Println(string(b))
		}
	}()

	if stats.Tracking.IPAddr.IsUnspecified() {
		log.Printf("No tracking source!")
		return sensu.CheckStateCritical, nil
	}

	log.Printf("Tracking server: %08X (%v)", stats.Tracking.RefID, stats.Tracking.IPAddr)
	if uint(stats.Tracking.Stratum) >= plugin.StratumCritical {
		log.Printf("CRITICAL: Tracking server stratum: %d", stats.Tracking.Stratum)
		return sensu.CheckStateCritical, nil
	} else if uint(stats.Tracking.Stratum) >= plugin.StratumWarning {
		log.Printf("WARNING: Tracking server stratum: %d", stats.Tracking.Stratum)
		return sensu.CheckStateWarning, nil
	}

	result := sensu.CheckStateOK
	setResult := func(newResult int) {
		if newResult > result {
			result = newResult
		}
	}

	reachability := 0.0
	for _, source := range stats.Sources {
		if source.Reachability == 255 {
			reachability += 100.0
		}

		// skip unreachable sources
		if source.State == chrony.SourceStateUnreach {
			continue
		}

		lastRx := uint(source.SinceSample)
		if lastRx >= plugin.LastRxCritical {
			log.Printf("CRITICAL: %v server LastRx: %d", source.IPAddr, lastRx)
			setResult(sensu.CheckStateCritical)
		} else if lastRx >= plugin.LastRxWarning {
			log.Printf("WARNING: %v server LastRx: %d", source.IPAddr, lastRx)
			setResult(sensu.CheckStateWarning)
		}
	}

	reachability /= float64(len(stats.Sources))
	if reachability <= plugin.ReachabilityCritical {
		log.Printf("CRITICAL: only %.1f%% sources reachable!", reachability)
		setResult(sensu.CheckStateCritical)
	} else if reachability <= plugin.ReachabilityWarning {
		log.Printf("WARNING: only %.1f%% sources reachable!", reachability)
		setResult(sensu.CheckStateWarning)
	}

	if result == sensu.CheckStateOK {
		log.Printf("Chrony status: OK")
	}

	return result, nil
}

type stats struct {
	Tracking chrony.Tracking
	Sources  []chrony.SourceData
}

func getStats(socketPath string) (*stats, error) {
	addr, err := net.ResolveUnixAddr("unixgram", socketPath)
	if err != nil {
		return nil, fmt.Errorf("socket address error: %w", err)
	}

	base, _ := path.Split(socketPath)
	local := path.Join(base, fmt.Sprintf("chronyc.%d.sock", os.Getpid()))
	localAddr, _ := net.ResolveUnixAddr("unixgram", local)

	sock, err := net.DialUnix("unixgram", localAddr, addr)
	if err != nil {
		return nil, fmt.Errorf("socket open error: %w", err)
	}
	defer sock.Close()
	defer os.RemoveAll(local)

	err = os.Chmod(local, 0666)
	if err != nil {
		return nil, fmt.Errorf("socket chmod 0666 error: %w", err)
	}

	// Suggest that IO shouldn't ever reach so long timeout
	err = sock.SetDeadline(time.Now().Add(time.Second))
	if err != nil {
		return nil, fmt.Errorf("socket deadline error: %w", err)
	}

	client := chrony.Client{
		Sequence:   1,
		Connection: sock,
	}

	stats := &stats{
		Sources: make([]chrony.SourceData, 0),
	}

	trReq := chrony.NewTrackingPacket()
	resp, err := client.Communicate(trReq)
	if err != nil {
		return nil, fmt.Errorf("tracking request error: %w", err)
	}
	tracking, ok := resp.(*chrony.ReplyTracking)
	if !ok {
		return nil, fmt.Errorf("unexpected reply: %v", resp)
	}
	stats.Tracking = tracking.Tracking

	sourcesReq := chrony.NewSourcesPacket()
	resp, err = client.Communicate(sourcesReq)
	if err != nil {
		return nil, fmt.Errorf("sources request error: %w", err)
	}
	sources, ok := resp.(*chrony.ReplySources)
	if !ok {
		return nil, fmt.Errorf("unexpected reply: %v", resp)
	}

	for i := 0; i < sources.NSources; i++ {
		srcReq := chrony.NewSourceDataPacket(int32(i))
		resp, err = client.Communicate(srcReq)
		if err != nil {
			return nil, fmt.Errorf("source data #%d request error: %w", i, err)
		}
		source, ok := resp.(*chrony.ReplySourceData)
		if !ok {
			return nil, fmt.Errorf("unexpected reply: %v", resp)
		}

		stats.Sources = append(stats.Sources, source.SourceData)
	}

	return stats, nil
}
