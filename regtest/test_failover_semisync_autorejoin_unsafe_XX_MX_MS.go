// replication-manager - Replication Manager Monitoring and CLI for MariaDB and MySQL
// Authors: Guillaume Lefranc <guillaume@signal18.io>
//          Stephane Varoqui  <stephane@mariadb.com>
// This source code is licensed under the GNU General Public License, version 3.

package regtest

import (
	"sync"
	"time"

	"github.com/tanji/replication-manager/cluster"
)

func testFailoverSemisyncAutoRejoinUnsafeMSMXXXRMXMS(cluster *cluster.Cluster, conf string, test string) bool {

	if cluster.InitTestCluster(conf, test) == false {
		return false
	}
	cluster.SetFailoverCtr(0)
	cluster.SetFailSync(false)
	cluster.SetInteractive(false)
	cluster.SetRplChecks(false)
	cluster.SetRejoin(true)
	cluster.SetRejoinFlashback(true)
	cluster.SetRejoinDump(true)
	cluster.EnableSemisync()
	cluster.SetFailTime(0)
	cluster.SetFailRestartUnsafe(true)
	cluster.SetBenchMethod("table")
	SaveMaster := cluster.GetMaster()

	cluster.CleanupBench()
	cluster.PrepareBench()
	go cluster.RunBench()
	time.Sleep(4 * time.Second)
	SaveMaster2 := cluster.GetSlaves()[0]

	cluster.KillMariaDB(cluster.GetSlaves()[0])
	time.Sleep(5 * time.Second)
	cluster.RunBench()

	cluster.KillMariaDB(cluster.GetMaster())
	time.Sleep(15 * time.Second)

	cluster.ForgetTopology()

	wg2 := new(sync.WaitGroup)
	wg2.Add(1)
	go cluster.WaitRejoin(wg2)
	cluster.StartMariaDB(SaveMaster)
	wg2.Wait()
	//Recovered as slave first wait that it trigger master failover
	time.Sleep(5 * time.Second)
	cluster.RunBench()

	wg2.Add(1)
	go cluster.WaitRejoin(wg2)
	cluster.StartMariaDB(SaveMaster2)
	wg2.Wait()
	time.Sleep(5 * time.Second)
	for _, s := range cluster.GetSlaves() {
		if s.IOThread != "Yes" || s.SQLThread != "Yes" {
			cluster.LogPrintf("ERROR", "Slave  %s issue on replication  SQL Thread % IO %s ", s.URL, s.SQLThread, s.IOThread)
			cluster.CloseTestCluster(conf, test)
			return false
		}
	}
	time.Sleep(10 * time.Second)
	if cluster.ChecksumBench() != true {
		cluster.LogPrintf("ERROR", "Inconsitant slave")
		cluster.CloseTestCluster(conf, test)
		return false
	}
	if len(cluster.GetServers()) == 2 && SaveMaster.URL != cluster.GetMaster().URL {
		cluster.LogPrintf("ERROR", "Unexpected master for 2 nodes cluster")
		return false
	}

	cluster.CloseTestCluster(conf, test)

	return true
}
