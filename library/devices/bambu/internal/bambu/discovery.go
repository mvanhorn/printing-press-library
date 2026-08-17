package bambu

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const SSDPService = "urn:bambulab-com:device:3dprinter:1"

var serialPattern = regexp.MustCompile(`^[A-Za-z0-9]{12,20}$`)

func ValidateSerial(serial string) error {
	if !serialPattern.MatchString(strings.TrimSpace(serial)) {
		return fmt.Errorf("printer serial must contain 12-20 letters or digits")
	}
	return nil
}

func ValidatePrivateIP(host string) error {
	ip := net.ParseIP(strings.TrimSpace(host))
	if ip == nil || !ip.IsPrivate() {
		return fmt.Errorf("printer host must be a literal private-LAN IP address")
	}
	return nil
}

func Discover(ctx context.Context, wantedSerial string) ([]Discovery, error) {
	wantedSerial = strings.ToUpper(strings.TrimSpace(wantedSerial))
	if wantedSerial != "" {
		if err := ValidateSerial(wantedSerial); err != nil {
			return nil, fmt.Errorf("BAMBU_SERIAL: %w", err)
		}
	}
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		return nil, fmt.Errorf("open SSDP socket: %w", err)
	}
	defer conn.Close()
	request := strings.Join([]string{
		"M-SEARCH * HTTP/1.1",
		"HOST: 239.255.255.250:1900",
		`MAN: "ssdp:discover"`,
		"MX: 2",
		"ST: " + SSDPService,
		"", "",
	}, "\r\n")
	for _, port := range []int{1900, 1990, 2021} {
		if _, err := conn.WriteToUDP([]byte(request), &net.UDPAddr{IP: net.IPv4(239, 255, 255, 250), Port: port}); err != nil {
			return nil, fmt.Errorf("send SSDP probe: %w", err)
		}
	}
	seen := map[string]bool{}
	results := make([]Discovery, 0)
	buffer := make([]byte, 65535)
	end := time.Now().Add(4 * time.Second)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(end) {
		end = ctxDeadline
	}
	for {
		deadline := time.Now().Add(400 * time.Millisecond)
		if end.Before(deadline) {
			deadline = end
		}
		_ = conn.SetReadDeadline(deadline)
		n, _, err := conn.ReadFromUDP(buffer)
		if err != nil {
			if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
				if ctx.Err() != nil || !time.Now().Before(end) {
					break
				}
				continue
			}
			return nil, fmt.Errorf("read SSDP response: %w", err)
		}
		discovered, ok := ParseSSDP(buffer[:n])
		if !ok || (wantedSerial != "" && !strings.EqualFold(discovered.Serial, wantedSerial)) || seen[discovered.Serial] {
			continue
		}
		seen[discovered.Serial] = true
		results = append(results, discovered)
		if wantedSerial != "" {
			return results, nil
		}
	}
	return results, nil
}

func ParseSSDP(payload []byte) (Discovery, bool) {
	normalized := strings.ReplaceAll(strings.ReplaceAll(string(payload), "\r\n", "\n"), "\r", "\n")
	reader := bufio.NewReader(strings.NewReader(normalized))
	status, err := reader.ReadString('\n')
	if err != nil || !strings.HasPrefix(status, "HTTP/") {
		return Discovery{}, false
	}
	response := &http.Response{Header: make(http.Header)}
	for {
		line, err := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "" || err != nil {
			break
		}
		key, value, found := strings.Cut(line, ":")
		if found {
			response.Header.Add(strings.TrimSpace(key), strings.TrimSpace(value))
		}
	}
	if !strings.EqualFold(strings.TrimSpace(response.Header.Get("ST")), SSDPService) {
		return Discovery{}, false
	}
	serial := strings.TrimSpace(response.Header.Get("DevSerial.bambu.com"))
	if serial == "" {
		serial = strings.TrimSpace(response.Header.Get("USN"))
	}
	if !serialPattern.MatchString(serial) {
		return Discovery{}, false
	}
	location := strings.TrimSpace(response.Header.Get("Location"))
	parsed, _ := url.Parse(location)
	host := parsed.Hostname()
	if host == "" {
		host = strings.Split(location, ":")[0]
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsPrivate() {
		return Discovery{}, false
	}
	return Discovery{
		Host: host, Serial: serial,
		Model: truncate(strings.TrimSpace(response.Header.Get("DevModel.bambu.com")), 32),
		Name:  truncate(SanitizeJobName(response.Header.Get("DevName.bambu.com")), 64),
	}, true
}
