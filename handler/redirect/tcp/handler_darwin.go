//go:build darwin

package redirect

import (
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
)

func (h *redirectHandler) getOriginalDstAddr(conn net.Conn) (addr net.Addr, err error) {
	host, port, err := LocalToRemote(conn)
	if err != nil {
		return nil, err
	}
	addr = &net.TCPAddr{
		IP:   net.ParseIP(host),
		Port: port,
	}

	return
}

func LocalToRemote(clientConn net.Conn) (string, int, error) {
	sep := strings.Split(clientConn.RemoteAddr().String(), ":")
	port, _ := strconv.Atoi(sep[1])
	remoteAddr, remotePort, err := PfctlLookup(sep[0], port)
	if err != nil {
		return "", 0, err
	}
	return remoteAddr, remotePort, err
}

func PfctlLookup(address string, port int) (string, int, error) {
	out, err := exec.Command("sudo", "-n", "/sbin/pfctl", "-s", "state").Output()
	if err != nil {
		panic(err)
	}

	return lookup(address, port, string(out))

}

func lookup(address string, port int, s string) (string, int, error) {
	// We may get an ipv4-mapped ipv6 address here, e.g. ::ffff:127.0.0.1.
	// Those still appear as "127.0.0.1" in the table, so we need to strip the prefix.
	// re := regexp.MustCompile(`^::ffff:((\d+\.\d+\.\d+\.\d+$))`)
	// strippedAddress := re.ReplaceAllString(address, "")
	strippedAddress := address

	// ALL tcp 192.168.1.13:57474 -> 23.205.82.58:443       ESTABLISHED:ESTABLISHED
	specv4 := fmt.Sprintf("%s:%d", strippedAddress, port)

	// ALL tcp 2a01:e35:8bae:50f0:9d9b:ef0d:2de3:b733[58505] -> 2606:4700:30::681f:4ad0[443]       ESTABLISHED:ESTABLISHED
	specv6 := fmt.Sprintf("%s[%d]", strippedAddress, port)

	lines := strings.Split(s, "\n")
	for _, line := range lines {
		if strings.Contains(line, "ESTABLISHED:ESTABLISHED") {
			if strings.Contains(line, specv4) {
				fields := strings.Fields(line)
				if len(fields) > 4 {
					addressPort := strings.Split(fields[4], ":")
					if len(addressPort) == 2 {
						return addressPort[0], convertPortToInt(addressPort[1]), nil
					}
				}
			} else if strings.Contains(line, specv6) {
				fields := strings.Fields(line)
				if len(fields) > 4 {
					portPart := strings.Split(fields[4], "[")
					portNumber := strings.Split(portPart[1], "]")[0]
					return portPart[0], convertPortToInt(portNumber), nil
				}
			}
		}
	}

	return "", 0, fmt.Errorf("could not resolve original destination")
}

func convertPortToInt(port string) int {
	result, err := strconv.Atoi(port)
	if err != nil {
		fmt.Println("Error converting port to int:", err)
	}
	return result
}
