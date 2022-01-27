package environment

import (
	"fmt"
	"net/url"
	"sort"

	"github.com/pkg/errors"
)

// Protocol represents a URL scheme to use when fetching connection details
type Protocol int

const (
	// WS : Web Socket Protocol
	WS Protocol = iota
	// WSS : Web Socket Secure Protocol
	WSS
	// HTTP : Hypertext Transfer Protocol
	HTTP
	// HTTPS : Hypertext Transfer Protocol Secure
	HTTPS
)

// ChartConnection info about connected pod ports
type ChartConnection struct {
	PodName     string         `yaml:"pod_name,omitempty" json:"pod_name" envconfig:"pod_name"`
	PodIP       string         `yaml:"pod_ip,omitempty" json:"pod_ip" envconfig:"pod_ip"`
	RemotePorts map[string]int `yaml:"remote_ports,omitempty" json:"remote_ports" envconfig:"remote_ports"`
	LocalPorts  map[string]int `yaml:"local_ports,omitempty" json:"local_ports" envconfig:"local_ports"`
}

// ChartConnections represents a group of pods and their connection info deployed within the same chart
type ChartConnections map[string]*ChartConnection

// Range emulates the default range function in the sync.Map, without the need to cast the key & value
func (cc ChartConnections) Range(f func(key string, chartConnection *ChartConnection) bool) {
	for k, v := range cc {
		if !f(k, v) {
			return
		}
	}
}

// Store emulates the default Store function within the sync.Map to use the common map key and value types and
// return an error if the key is a duplicate
func (cc ChartConnections) Store(app, instance, name string, chartConnection *ChartConnection) error {
	mapKey := cc.mapKey(app, instance, name)
	cc[mapKey] = chartConnection
	return nil
}

// Load emulates the Load sync.Map function to use the common map key and return the value correctly typed
func (cc ChartConnections) Load(app, instance, name string) (*ChartConnection, error) {
	mapKey := cc.mapKey(app, instance, name)
	if _, ok := cc[mapKey]; !ok {
		return nil, fmt.Errorf("chart connection by the key of '%s' doesn't exist", mapKey)
	}
	return cc[mapKey], nil
}

// LoadByPort scans all the connections and returns a list of connections if they contain a certain port number
func (cc *ChartConnections) LoadByPort(port int) ([]*ChartConnection, error) {
	var connections []*ChartConnection
	cc.Range(func(_ string, chartConnection *ChartConnection) bool {
		for _, podPort := range chartConnection.RemotePorts {
			if port == podPort {
				connections = append(connections, chartConnection)
			}
		}
		return true
	})
	if len(connections) == 0 {
		return connections, fmt.Errorf("no connections by port %d found in the environment", port)
	}
	return connections, nil
}

// LoadByPortName scans all the connections and returns a list of connections if they contain a certain port name
func (cc *ChartConnections) LoadByPortName(portName string) ([]*ChartConnection, error) {
	var connections []*ChartConnection
	cc.Range(func(_ string, chartConnection *ChartConnection) bool {
		for remotePortName := range chartConnection.RemotePorts {
			if remotePortName == portName {
				connections = append(connections, chartConnection)
			}
		}
		return true
	})
	if len(connections) == 0 {
		return connections, fmt.Errorf("no connections by port %s found in the environment", portName)
	}

	// Ensures that when calling either for local or remote ports, the pods are returned in the same order. This enables
	// matching of Local and Remote URLs.
	sort.Slice(connections, func(i, j int) bool {
		return connections[i].PodIP < connections[j].PodIP
	})

	return connections, nil
}

// RemoteURLsByPort returns parsed URLs of a remote service based on the port name
func (cc *ChartConnections) RemoteURLsByPort(portName string, protocol Protocol) ([]*url.URL, error) {
	switch protocol {
	case WS:
		return cc.RemoteURLs("ws://%s:%d", portName)
	case WSS:
		return cc.RemoteURLs("wss://%s:%d", portName)
	case HTTP:
		return cc.RemoteURLs("http://%s:%d", portName)
	case HTTPS:
		return cc.RemoteURLs("https://%s:%d", portName)
	default:
		return nil, errors.New("no such protocol")
	}
}

// RemoteURLByPort returns a parsed URL of a remote service based on the port name
func (cc *ChartConnections) RemoteURLByPort(portName string, protocol Protocol) (*url.URL, error) {
	if urls, err := cc.RemoteURLsByPort(portName, protocol); err != nil {
		return nil, err
	} else {
		return urls[0], nil
	}
}

// LocalURLsByPort returns parsed URLs of a local port-forwarded service based on the port name
func (cc *ChartConnections) LocalURLsByPort(portName string, protocol Protocol) ([]*url.URL, error) {
	switch protocol {
	case WS:
		return cc.LocalURLs("ws://localhost:%d", portName)
	case WSS:
		return cc.LocalURLs("wss://localhost:%d", portName)
	case HTTP:
		return cc.LocalURLs("http://localhost:%d", portName)
	case HTTPS:
		return cc.LocalURLs("https://localhost:%d", portName)
	default:
		return nil, errors.New("no such protocol")
	}
}

// LocalURLByPort returns a parsed URL of a local port-forwarded service based on the port name
func (cc *ChartConnections) LocalURLByPort(portName string, protocol Protocol) (*url.URL, error) {
	if urls, err := cc.LocalURLsByPort(portName, protocol); err != nil {
		return nil, err
	} else {
		return urls[0], nil
	}
}

// RemoteURLs scans all the connections returns remote URLs based on a port number of a service and a string directive
func (cc *ChartConnections) RemoteURLs(stringDirective string, portName string) ([]*url.URL, error) {
	var urls []*url.URL
	connections, err := cc.LoadByPortName(portName)
	if err != nil {
		return urls, err
	}
	for _, connection := range connections {
		for remotePortName, remotePort := range connection.RemotePorts {
			if remotePortName == portName {
				parsedURL, err := url.Parse(fmt.Sprintf(stringDirective, connection.PodIP, remotePort))
				if err != nil {
					return nil, err
				}
				urls = append(urls, parsedURL)
			}
		}
	}
	return urls, nil
}

// LocalURLs scans all the connections returns remote URLs based on a port number of a service and a string directive
func (cc *ChartConnections) LocalURLs(stringDirective string, portName string) ([]*url.URL, error) {
	var urls []*url.URL
	connections, err := cc.LoadByPortName(portName)
	if err != nil {
		return nil, err
	}
	for _, connection := range connections {
		for remotePortName := range connection.RemotePorts {
			if remotePortName == portName {
				localPort, ok := connection.LocalPorts[remotePortName]
				if !ok {
					return urls, fmt.Errorf("local port for service doesn't exist, must not be connected")
				}
				parsedURL, err := url.Parse(fmt.Sprintf(stringDirective, localPort))
				if err != nil {
					return nil, err
				}
				urls = append(urls, parsedURL)
			}
		}
	}
	return urls, nil
}

func (cc *ChartConnections) mapKey(app, instance, name string) string {
	return fmt.Sprintf("%s_%s_%s", app, instance, name)
}
