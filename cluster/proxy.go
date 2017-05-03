package cluster

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tanji/replication-manager/maxscale"
	"github.com/tanji/replication-manager/misc"
	"github.com/tanji/replication-manager/state"
)

// Proxy defines a proxy
type Proxy struct {
	Name          string
	Host          string
	Port          string
	User          string
	Pass          string
	WritePort     int
	ReadPort      int
	ReadWritePort int
}

const (
	proxyMaxscale string = "maxscale"
	proxyHaproxy  string = "haproxy"
	proxySqlproxy string = "sqlproxy"
	proxySpider   string = "mdbsproxy"
)

type proxyList []*Proxy

func (cluster *Cluster) newProxyList() error {
	nbproxies := 0
	if cluster.conf.MxsHost != "" && cluster.conf.MxsOn {
		nbproxies += len(strings.Split(cluster.conf.MxsHost, ","))
	}
	if cluster.conf.HaproxyOn {
		nbproxies++
	}
	if cluster.conf.MdbsProxyHosts != "" && cluster.conf.MdbsProxyOn {
		nbproxies += len(strings.Split(cluster.conf.MdbsProxyHosts, ","))
	}

	cluster.proxies = make([]*Proxy, nbproxies)

	cluster.LogPrintf("INFO: Loading %d proxies", nbproxies)

	var ctproxy = 0
	var err error
	if cluster.conf.MxsHost != "" && cluster.conf.MxsOn {
		for _, proxyHost := range strings.Split(cluster.conf.MxsHost, ",") {
			cluster.LogPrintf("INFO: Loading Maxscale...")
			prx := new(Proxy)
			prx.Name = proxyMaxscale
			prx.Host = proxyHost
			prx.Port = cluster.conf.MxsPort
			prx.User = cluster.conf.MxsUser
			prx.Pass = cluster.conf.MxsPass
			prx.ReadPort = cluster.conf.MxsReadPort
			prx.WritePort = cluster.conf.MxsWritePort

			cluster.proxies[ctproxy], err = cluster.newProxy(prx)
			if err != nil {
				cluster.LogPrintf("ERROR: Could not open connection to proxy %s %s: %s", prx.Host, prx.Port, err)
			}
			if cluster.conf.Verbose {
				cluster.tlog.Add(fmt.Sprintf("[%s] DEBUG: New proxy created: %s ,%s", cluster.cfgGroup, prx.Host, prx.Port))
			}
			ctproxy++
		}
	}
	if cluster.conf.HaproxyOn {
		cluster.LogPrintf("INFO: Loading HaProxy...")

		prx := new(Proxy)
		prx.Name = proxyHaproxy
		prx.Port = strconv.Itoa(cluster.conf.HaproxyStatPort)
		prx.Host = cluster.conf.HaproxyWriteBindIp
		prx.ReadPort = cluster.conf.HaproxyReadPort
		prx.WritePort = cluster.conf.HaproxyWritePort
		prx.ReadWritePort = cluster.conf.HaproxyWritePort
		cluster.proxies[ctproxy], err = cluster.newProxy(prx)
		if err != nil {
			cluster.LogPrintf("ERROR: Could not open connection to proxy %s %s: %s", prx.Host, prx.Port, err)
		}
		if cluster.conf.Verbose {
			cluster.tlog.Add(fmt.Sprintf("[%s] DEBUG: New proxy created: %s ,%s", cluster.cfgGroup, prx.Host, prx.Port))
		}
		ctproxy++
	}
	if cluster.conf.MdbsProxyHosts != "" && cluster.conf.MdbsProxyOn {
		for _, proxyHost := range strings.Split(cluster.conf.MdbsProxyHosts, ",") {
			cluster.LogPrintf("INFO: Loading MdbShardProxy...")
			prx := new(Proxy)
			prx.Name = proxySpider
			prx.Host, prx.Port = misc.SplitHostPort(proxyHost)
			prx.User, prx.Pass = misc.SplitPair(cluster.conf.MdbsProxyUser)
			prx.ReadPort, _ = strconv.Atoi(prx.Port)
			prx.WritePort, _ = strconv.Atoi(prx.Port)
			prx.ReadWritePort, _ = strconv.Atoi(prx.Port)
			cluster.proxies[ctproxy], err = cluster.newProxy(prx)
			if err != nil {
				cluster.LogPrintf("ERROR: Could not open connection to proxy %s %s: %s", prx.Host, prx.Port, err)
			}
			if cluster.conf.Verbose {
				cluster.tlog.Add(fmt.Sprintf("[%s] DEBUG: New proxy created: %s ,%s", cluster.cfgGroup, prx.Host, prx.Port))
			}
			ctproxy++
		}
	}

	return nil
}

func (cluster *Cluster) newProxy(p *Proxy) (*Proxy, error) {
	proxy := new(Proxy)
	proxy = p
	return proxy, nil
}

func (cluster *Cluster) SetMaintenance(serverid string) {
	// Found server from ServerId
	for _, pr := range cluster.proxies {
		if cluster.conf.MxsOn && pr.Name == "maxscale" {
			intsrvid, _ := strconv.Atoi(serverid)
			server := cluster.GetServerFromId(uint(intsrvid))
			for _, p := range cluster.proxies {
				if cluster.master != nil {
					if p.Name == proxyMaxscale {
						m := maxscale.MaxScale{Host: pr.Host, Port: pr.Port, User: pr.User, Pass: pr.Pass}
						err := m.Connect()
						if err != nil {
							cluster.sme.AddState("ERR00018", state.State{ErrType: "ERROR", ErrDesc: fmt.Sprintf(clusterError["ERR00018"], err), ErrFrom: "CONF"})
						}
						err = m.SetServer(server.MxsServerName, "maintenance")
						if err != nil {
							cluster.LogPrintf("ERROR: Could not set  server %s in maintenance", err)
							m.Close()
						}
						m.Close()
					}
				}
			}
		}
	}
}

func (cluster *Cluster) refreshProxies() {
	for _, pr := range cluster.proxies {
		if cluster.conf.MxsOn && pr.Name == proxyMaxscale {
			for _, server := range cluster.servers {
				if server.PrevState != server.State {
					cluster.initMaxscale(nil, pr)
					break
				}
			}
		}
		if cluster.conf.MdbsProxyOn && pr.Name == proxySpider {
			if cluster.GetStateMachine().GetHeartbeats()%60 == 0 {
				cluster.refreshMdbsproxy(nil, pr)

			}
		}
	}

}
func (cluster *Cluster) failoverProxies() {
	cluster.initProxies()
}

func (cluster *Cluster) initProxies() {
	for _, pr := range cluster.proxies {
		cluster.LogPrintf("Init %s %s %s", pr.Name, pr.Host, pr.Port)
		if cluster.conf.HaproxyOn && pr.Name == proxyHaproxy {
			cluster.initHaproxy(nil, pr)
		}
		if cluster.conf.MxsOn && pr.Name == proxyMaxscale {
			cluster.initMaxscale(nil, pr)
		}
		if cluster.conf.MdbsProxyOn && pr.Name == proxySpider {
			cluster.initMdbsproxy(nil, pr)
		}
	}
}