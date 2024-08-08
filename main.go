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
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/sardinasystems/sensu-go-check-common/nagios"
	corev2 "github.com/sensu/core/v2"
	"github.com/sensu/sensu-plugin-sdk/sensu"
)

// Config represents the check plugin config.
type Config struct {
	sensu.PluginConfig
	SocketPath           string
	StratumWarning       nagios.Threshold
	StratumCritical      nagios.Threshold
	ReachabilityWarning  nagios.Threshold
	ReachabilityCritical nagios.Threshold
	SourcesWarning       nagios.Threshold
	SourcesCritical      nagios.Threshold
	LastrxWarning        nagios.Threshold
	LastrxCritical       nagios.Threshold
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

	options = []sensu.ConfigOption{
		&sensu.PluginConfigOption[string]{
			Path:      "socket",
			Env:       "HAPROXY_SOCKET",
			Argument:  "socket",
			Shorthand: "S",
			Default:   chrony.ChronySocketPath,
			Usage:     "Path to haproxy control socket",
			Value:     &plugin.SocketPath,
		},
		&nagios.ThresholdConfigOption{
			Option: sensu.PluginConfigOption[string]{
				Path:     "stratum_warning",
				Env:      "CHRONY_STRATUM_WARNING",
				Argument: "stratum-warning",
				Default:  "@10:",
				Usage:    "Stratum warning level [threshold]",
			},
			Value: &plugin.StratumWarning,
		},
		&nagios.ThresholdConfigOption{
			Option: sensu.PluginConfigOption[string]{
				Path:     "stratum_critical",
				Env:      "CHRONY_STRATUM_CRITICAL",
				Argument: "stratum-critical",
				Default:  "@12:",
				Usage:    "Stratum critical level [threshold]",
			},
			Value: &plugin.StratumCritical,
		},
		&nagios.ThresholdConfigOption{
			Option: sensu.PluginConfigOption[string]{
				Path:      "reachability_warning",
				Env:       "CHRONY_REACHABILITY_WARNING",
				Argument:  "reachablility-warning",
				Shorthand: "w",
				Default:   "@~:67.0",
				Usage:     "Reachablility warning percent [threshold]",
			},
			Value: &plugin.ReachabilityWarning,
		},
		&nagios.ThresholdConfigOption{
			Option: sensu.PluginConfigOption[string]{
				Path:      "reachability_critical",
				Env:       "CHRONY_REACHABILITY_CRITICAL",
				Argument:  "reachablility-critical",
				Shorthand: "c",
				Default:   "@~:34.0",
				Usage:     "Reachablility critical percent [threshold]",
			},
			Value: &plugin.ReachabilityCritical,
		},
		&nagios.ThresholdConfigOption{
			Option: sensu.PluginConfigOption[string]{
				Path:     "sources_warning",
				Env:      "CHRONY_SOURCES_WARNING",
				Argument: "sources-warning",
				Default:  "3:",
				Usage:    "Minimal good sources [threshold]",
			},
			Value: &plugin.SourcesWarning,
		},
		&nagios.ThresholdConfigOption{
			Option: sensu.PluginConfigOption[string]{
				Path:     "sources_critical",
				Env:      "CHRONY_SOURCES_CRITICAL",
				Argument: "sources-critical",
				Default:  "1:",
				Usage:    "Minimal good sources [threshold]",
			},
			Value: &plugin.SourcesCritical,
		},
		&nagios.ThresholdConfigOption{
			Option: sensu.PluginConfigOption[string]{
				Path:      "lastrx_warning",
				Env:       "CHRONY_LASTRX_WARNING",
				Argument:  "lastrx-warning",
				Shorthand: "W",
				Default:   "64",
				Usage:     "Check lastrx [s][threshold] (unused. Left only for opts compatibility)",
			},
			Value: &plugin.LastrxWarning,
		},
		&nagios.ThresholdConfigOption{
			Option: sensu.PluginConfigOption[string]{
				Path:      "lastrx_critical",
				Env:       "CHRONY_LASTRX_CRITICAL",
				Argument:  "lastrx-critical",
				Shorthand: "C",
				Default:   "128",
				Usage:     "Check lastrx [s][threshold] (unused. Left only for opts compatibility)",
			},
			Value: &plugin.LastrxCritical,
		},
		&sensu.PluginConfigOption[bool]{
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
	useStdin := false
	fi, err := os.Stdin.Stat()
	if err != nil {
		fmt.Printf("Error check stdin: %v\n", err)
		panic(err)
	}
	//Check the Mode bitmask for Named Pipe to indicate stdin is connected
	if fi.Mode()&os.ModeNamedPipe != 0 {
		useStdin = true
	}

	check := sensu.NewGoCheck(&plugin.PluginConfig, options, checkArgs, executeCheck, useStdin)
	check.Execute()
}

func checkArgs(event *corev2.Event) (int, error) {
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

func executeCheck(event *corev2.Event) (int, error) {
	stats, err := getStats(plugin.SocketPath)
	if err != nil {
		return sensu.CheckStateUnknown, err
	}

	return doCheck(event, stats)
}

func doCheck(event *corev2.Event, stats *Stats) (int, error) {
	result := sensu.CheckStateOK
	setResult := func(newResult int) {
		if newResult > result {
			result = newResult
		}
	}

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"FL", "Source IP", "Str", "State", "Last Rx", "Reachability", "Last sample", "Error"})

	// Print debug data on exit
	defer func() {
		t.Render()

		if plugin.Debug {
			b, _ := json.MarshalIndent(stats, "", "  ")
			fmt.Println(string(b))
		}
	}()

	if stats.Tracking.IPAddr.IsUnspecified() {
		log.Printf("CRITICAL: No tracking source!")
		setResult(sensu.CheckStateCritical)
	}

	if plugin.StratumCritical.Check(float64(stats.Tracking.Stratum)) {
		log.Printf("CRITICAL: Tracking server stratum: %d", stats.Tracking.Stratum)
		setResult(sensu.CheckStateCritical)
	} else if plugin.StratumWarning.Check(float64(stats.Tracking.Stratum)) {
		log.Printf("WARNING: Tracking server stratum: %d", stats.Tracking.Stratum)
		setResult(sensu.CheckStateWarning)
	}

	ftod := func(f float64) time.Duration {
		return time.Duration(f*1e9) * time.Nanosecond
	}

	reachability := 0.0
	reachableSources := 0
	for _, source := range stats.Sources {
		// Reachability is a bitmask, 8 bits
		// See also: https://chrony-project.org/doc/3.2/chronyc.html
		srcReach := 0.0
		for mask := uint16(1); mask < 0x100; mask <<= 1 {
			if (source.Reachability & mask) != 0 {
				srcReach += 100.0 / 8
			}
		}

		fl := ""
		row := table.Row{
			fl,
			source.IPAddr,
			source.Stratum,
			source.State,
			time.Duration(source.SinceSample) * time.Second,
			fmt.Sprintf("%.1f%% (0b%08b)", srcReach, source.Reachability),
			fmt.Sprintf("%s[%s]", ftod(source.LatestMeas), ftod(source.OrigLatestMeas)),
			ftod(source.LatestMeasErr),
		}

		// count only good sources: sync|candidate
		if !(source.State == chrony.SourceStateSync || source.State == chrony.SourceStateCandidate) {
			t.AppendRow(row)
			continue
		}

		reachability += srcReach
		reachableSources++

		if stats.Tracking.IPAddr.Equal(source.IPAddr) {
			fl += "*"
		}

		if srcReach < 100.0 {
			// log.Printf("WARNING: %v (%v) server reachability: %.1f%% (0b%08b)", source.IPAddr, source.State, srcReach, source.Reachability)
			fl += "W"
		}

		row[0] = fl
		t.AppendRow(row)
	}

	if reachableSources == 0 {
		log.Printf("CRITICAL: no sources reachable!")
		setResult(sensu.CheckStateCritical)
		return result, nil
	} else if plugin.SourcesCritical.Check(float64(reachableSources)) {
		log.Printf("CRITICAL: good sources %s ~= %d", plugin.SourcesCritical.String(), reachableSources)
		setResult(sensu.CheckStateCritical)
	} else if plugin.SourcesWarning.Check(float64(reachableSources)) {
		log.Printf("WARNING: good sources %s ~= %d", plugin.SourcesWarning.String(), reachableSources)
		setResult(sensu.CheckStateWarning)
	}

	reachability /= float64(reachableSources)
	if plugin.ReachabilityCritical.Check(reachability) {
		log.Printf("CRITICAL: only %.1f%% sources reachable!", reachability)
		setResult(sensu.CheckStateCritical)
	} else if plugin.ReachabilityWarning.Check(reachability) {
		log.Printf("WARNING: only %.1f%% sources reachable!", reachability)
		setResult(sensu.CheckStateWarning)
	}

	if result == sensu.CheckStateOK {
		log.Printf("Chrony status: OK")
	}

	return result, nil
}

type Stats struct {
	Tracking chrony.Tracking
	Sources  []chrony.SourceData
}

func getStats(socketPath string) (*Stats, error) {
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

	stats := &Stats{
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
