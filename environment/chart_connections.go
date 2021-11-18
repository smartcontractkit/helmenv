package environment

import (
	"fmt"
	"net/url"
)

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
	if _, ok := cc[mapKey]; ok {
		return fmt.Errorf(
			"chart connection key of '%s' is already stored in the map", cc.mapKey(app, instance, name),
		)
	}
	cc[mapKey] = chartConnection
	return nil
}

// Load emulates the Load sync.Map function to use the common map key and return the value correctly typed
func (cc ChartConnections) Load(app, instance, name string) (*ChartConnection, error) {
	mapKey := cc.mapKey(app, instance, name)
	if _, ok := cc[mapKey]; !ok {
		return nil, fmt.Errorf(
			"chart connection by the key of '%s' doesn't exist", cc.mapKey(app, instance, name),
		)
	}
	return cc[mapKey], nil
}

// LoadByPort scans all the connections and returns a list of connections if they contain a certain port number
func (cc *ChartConnections) LoadByPort(port int) ([]*ChartConnection, error) {
	var connections []*ChartConnection
	cc.Range(func(_ string, chartConnection *ChartConnection) bool {
		for _, podPort := range chartConnection.Ports {
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
		for remotePortName, _ := range chartConnection.Ports {
			if remotePortName == portName {
				connections = append(connections, chartConnection)
			}
		}
		return true
	})
	if len(connections) == 0 {
		return connections, fmt.Errorf("no connections by port %s found in the environment", portName)
	}
	return connections, nil
}

// RemoteHTTPURLsByPort scans all the connections returns remote URLs based on a port number of a service
func (cc *ChartConnections) RemoteHTTPURLsByPort(port int) ([]*url.URL, error) {
	return cc.RemoteURLsByPort("http://%s:%d", port)
}

// RemoteWSURLsByPort scans all the connections returns remote URLs based on a port number of a service
func (cc *ChartConnections) RemoteWSURLsByPort(port int) ([]*url.URL, error) {
	return cc.RemoteURLsByPort("ws://%s:%d", port)
}

// RemoteURLsByPort scans all the connections returns remote URLs based on a port number of a service and a string directive
func (cc *ChartConnections) RemoteURLsByPort(stringDirective string, port int) ([]*url.URL, error) {
	var urls []*url.URL
	connections, err := cc.LoadByPort(port)
	if err != nil {
		return urls, err
	}
	for _, connection := range connections {
		for _, remotePort := range connection.Ports {
			if remotePort == port {
				parsedURL, _ := url.Parse(fmt.Sprintf(stringDirective, connection.PodIP, remotePort))
				urls = append(urls, parsedURL)
			}
		}
	}
	return urls, nil
}

// LocalHTTPURLsByPort scans all the connections returns local URLs based on a remote port number of a service
func (cc *ChartConnections) LocalHTTPURLsByPort(port int) ([]*url.URL, error) {
	return cc.LocalURLsByPort("http://%s:%d", port)
}

// LocalWSURLsByPort scans all the connections returns local URLs based on a remote port number of a service
func (cc *ChartConnections) LocalWSURLsByPort(port int) ([]*url.URL, error) {
	return cc.LocalURLsByPort("ws://%s:%d", port)
}

// LocalURLsByPort scans all the connections returns remote URLs based on a port number of a service and a string directive
func (cc *ChartConnections) LocalURLsByPort(stringDirective string, port int) ([]*url.URL, error) {
	var urls []*url.URL
	connections, err := cc.LoadByPort(port)
	if err != nil {
		return nil, err
	}
	for _, connection := range connections {
		for k, remotePort := range connection.Ports {
			if remotePort == port {
				localPort, ok := connection.LocalPorts[k]
				if !ok {
					return urls, fmt.Errorf("local port for service doesn't exist, must not be connected")
				}
				parsedURL, _ := url.Parse(fmt.Sprintf(stringDirective, connection.PodIP, localPort))
				urls = append(urls, parsedURL)
			}
		}
	}
	return urls, nil
}

// RemoteHTTPURLs scans all the connections returns remote URLs based on a port name of a service
func (cc *ChartConnections) RemoteHTTPURLs(portName string) ([]*url.URL, error) {
	return cc.RemoteURLs("http://%s:%d", portName)
}

// RemoteWSURLs scans all the connections returns remote URLs based on a port name of a service
func (cc *ChartConnections) RemoteWSURLs(portName string) ([]*url.URL, error) {
	return cc.RemoteURLs("ws://%s:%d", portName)
}

// RemoteHTTPURL scans all the connections returns remote URLs based on a port name of a service
func (cc *ChartConnections) RemoteHTTPURL(portName string) (*url.URL, error) {
	urls, err := cc.RemoteURLs("http://%s:%d", portName)
	if err != nil {
		return nil, err
	}
	return urls[0], err
}

// RemoteWSURL scans all the connections returns remote URLs based on a port name of a service
func (cc *ChartConnections) RemoteWSURL(portName string) (*url.URL, error) {
	urls, err := cc.RemoteURLs("ws://%s:%d", portName)
	if err != nil {
		return nil, err
	}
	return urls[0], err
}

// RemoteURLs scans all the connections returns remote URLs based on a port number of a service and a string directive
func (cc *ChartConnections) RemoteURLs(stringDirective string, portName string) ([]*url.URL, error) {
	var urls []*url.URL
	connections, err := cc.LoadByPortName(portName)
	if err != nil {
		return urls, err
	}
	for _, connection := range connections {
		for remotePortName, remotePort := range connection.Ports {
			if remotePortName == portName {
				parsedURL, _ := url.Parse(fmt.Sprintf(stringDirective, connection.PodIP, remotePort))
				urls = append(urls, parsedURL)
			}
		}
	}
	return urls, nil
}

// LocalHTTPURLs scans all the connections returns local URLs based on a remote port name of a service
func (cc *ChartConnections) LocalHTTPURLs(portName string) ([]*url.URL, error) {
	return cc.LocalURLs("http://localhost:%d", portName)
}

// LocalWSURLs scans all the connections returns local URLs based on a remote port name of a service
func (cc *ChartConnections) LocalWSURLs(portName string) ([]*url.URL, error) {
	return cc.LocalURLs("ws://localhost:%d", portName)
}

// LocalHTTPURL scans all the connections returns local URL based on a remote port name of a service
func (cc *ChartConnections) LocalHTTPURL(portName string) (*url.URL, error) {
	urls, err := cc.LocalURLs("http://localhost:%d", portName)
	if err != nil {
		return nil, err
	}
	return urls[0], nil
}

// LocalWSURL scans all the connections returns local URL based on a remote port name of a service
func (cc *ChartConnections) LocalWSURL(portName string) (*url.URL, error) {
	urls, err := cc.LocalURLs("ws://localhost:%d", portName)
	if err != nil {
		return nil, err
	}
	return urls[0], nil
}

// LocalURLs scans all the connections returns remote URLs based on a port number of a service and a string directive
func (cc *ChartConnections) LocalURLs(stringDirective string, portName string) ([]*url.URL, error) {
	var urls []*url.URL
	connections, err := cc.LoadByPortName(portName)
	if err != nil {
		return nil, err
	}
	for _, connection := range connections {
		for remotePortName := range connection.Ports {
			if remotePortName == portName {
				localPort, ok := connection.LocalPorts[remotePortName]
				if !ok {
					return urls, fmt.Errorf("local port for service doesn't exist, must not be connected")
				}
				parsedURL, _ := url.Parse(fmt.Sprintf(stringDirective, localPort))
				urls = append(urls, parsedURL)
			}
		}
	}
	return urls, nil
}

func (cc *ChartConnections) mapKey(app, instance, name string) string {
	return fmt.Sprintf("%s_%s_%s", app, instance, name)
}

// ChartConnection info about connected pod ports
type ChartConnection struct {
	PodName      string         `yaml:"pod_name"`
	ForwarderPID int            `yaml:"forwarder_pid"`
	PodIP        string         `yaml:"pod_ip"`
	Ports        map[string]int `yaml:"ports"`
	LocalPorts   map[string]int `yaml:"local_port"`
}