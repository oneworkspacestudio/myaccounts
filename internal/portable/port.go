package portable

import "net"

func AvailablePort(preferred int) (int, error) {
	listener, e := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: preferred})
	if e != nil {
		listener, e = net.ListenTCP("tcp4", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	}
	if e != nil {
		return 0, e
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}
