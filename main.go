package main

import (
	"encoding/json"
	"fmt"
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
	// Jitter? Stratum?
	OffsetWarning  float64
	OffsetCritical float64
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
			Path:      "offset_warning",
			Env:       "CHRONY_OFFSET_WARNING",
			Argument:  "offset-warning",
			Shorthand: "w",
			Default:   0.050,
			Usage:     "Offset warning level [s]",
			Value:     &plugin.OffsetWarning,
		},
		{
			Path:      "offset_critical",
			Env:       "CHRONY_OFFSET_CRITICAL",
			Argument:  "offset-critical",
			Shorthand: "c",
			Default:   0.100,
			Usage:     "Offset critical level [s]",
			Value:     &plugin.OffsetCritical,
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

	_ = stats

	b, _ := json.MarshalIndent(stats, "", "  ")
	fmt.Println(string(b))

	return sensu.CheckStateOK, nil
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
	sock.SetDeadline(time.Now().Add(time.Second))

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
