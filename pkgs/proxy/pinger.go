package proxy

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/moqsien/neobox/pkgs/conf"
	"github.com/moqsien/neobox/pkgs/utils/log"
	probing "github.com/prometheus-community/pro-bing"
)

type NeoPinger struct {
	conf       *conf.NeoBoxConf
	pingedList *ProxyList
	fetcher    *Fetcher
	sendChan   chan *Proxy
	wg         sync.WaitGroup
}

func NewNeoPinger(cnf *conf.NeoBoxConf) *NeoPinger {
	fPath := filepath.Join(cnf.NeoWorkDir, cnf.PingedFileName)
	return &NeoPinger{
		conf:       cnf,
		pingedList: NewProxyList(fPath),
		fetcher:    NewFetcher(cnf),
		wg:         sync.WaitGroup{},
	}
}

func (that *NeoPinger) Send() {
	that.sendChan = make(chan *Proxy, 30)
	r := that.fetcher.GetRawProxyList(false)
	fmt.Printf("find %v raw proxies.", len(r))
	for _, rawUri := range r {
		p := DefaultProxyPool.Get(rawUri)
		if p != nil {
			that.sendChan <- p
		}
	}
	close(that.sendChan)
}

func (that *NeoPinger) ping(p *Proxy) {
	if p != nil {
		if pinger, err := probing.NewPinger(p.Address()); err == nil {
			if runtime.GOOS == "windows" {
				pinger.SetPrivileged(true)
			}
			pinger.Count = 5
			pinger.Interval = time.Millisecond * 300
			pinger.Timeout = time.Second * 2
			pinger.OnFinish = func(s *probing.Statistics) {
				if s.PacketLoss < 10.0 {
					p.RTT = s.AvgRtt.Milliseconds()
					if p.RTT <= that.conf.MaxAvgRTT {
						that.pingedList.AddProxies(p)
					}
				} else {
					fmt.Println("ping failed: ", p.String())
				}
			}
			if err := pinger.Run(); err != nil {
				log.PrintError(err)
			}
		}
	}
}

func (that *NeoPinger) Ping() {
	that.wg.Add(1)
	defer that.wg.Done()
	for {
		select {
		case p, ok := <-that.sendChan:
			if p == nil && !ok {
				return
			}
			that.ping(p)
		default:
			time.Sleep(time.Millisecond * 100)
		}
	}
}

func (that *NeoPinger) Run() *ProxyList {
	go that.Send()
	time.Sleep(time.Millisecond * 100)
	that.pingedList.Clear()
	for i := 0; i < that.conf.MaxPingers; i++ {
		go that.Ping()
	}
	that.wg.Wait()
	if that.pingedList.Len() > 0 {
		that.pingedList.Save()
	}
	return that.pingedList
}

/*
Set pinger for Unix
*/
func SetPingWithoutRootForUnix() {
	// sudo sysctl -w net.ipv4.ping_group_range="0 2147483647"
	cmd := exec.Command("sudo", "sysctl", "-w", `net.ipv4.ping_group_range="0 2147483647"`)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		log.PrintError(err)
	}
}
