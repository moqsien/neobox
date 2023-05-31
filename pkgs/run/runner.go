package run

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	socks "github.com/moqsien/goutils/pkgs/socks"
	futils "github.com/moqsien/goutils/pkgs/utils"
	"github.com/moqsien/neobox/pkgs/clients"
	"github.com/moqsien/neobox/pkgs/conf"
	"github.com/moqsien/neobox/pkgs/iface"
	"github.com/moqsien/neobox/pkgs/proxy"
	"github.com/moqsien/neobox/pkgs/utils/log"
	cron "github.com/robfig/cron/v3"
)

const (
	ExtraSockName     = "neobox_ping.sock"
	KtrlShellSockName = "neobox_ktrl.sock"
	OkStr             = "ok"
	runnerPingRoute   = "pingRunner"
	winRunScriptName  = "neobox_runner.bat"
)

var StopChan chan struct{} = make(chan struct{})

type Runner struct {
	verifier   *proxy.Verifier
	conf       *conf.NeoBoxConf
	client     iface.IClient
	extraSocks string
	pingClient *socks.UClient
	daemon     *futils.Daemon
	cron       *cron.Cron
}

func NewRunner(cnf *conf.NeoBoxConf) *Runner {
	r := &Runner{
		verifier:   proxy.NewVerifier(cnf),
		conf:       cnf,
		extraSocks: ExtraSockName,
		daemon:     futils.NewDaemon(),
		cron:       cron.New(),
	}
	r.daemon.SetWorkdir(cnf.NeoWorkDir)
	r.daemon.SetScriptName(winRunScriptName)
	return r
}

func (that *Runner) startRunnerPingServer() {
	server := socks.NewUServer(that.extraSocks)
	server.AddHandler(runnerPingRoute, func(c *gin.Context) {
		c.String(http.StatusOK, OkStr)
	})
	if err := server.Start(); err != nil {
		log.PrintError("[start ping server failed] ", err)
	}
}

func (that *Runner) Ping() bool {
	if that.pingClient == nil {
		that.pingClient = socks.NewUClient(that.extraSocks)
	}
	if resp, err := that.pingClient.GetResp(runnerPingRoute, map[string]string{}); err == nil {
		return strings.Contains(resp, OkStr)
	}
	return false
}

func (that *Runner) Start() {
	if that.Ping() {
		fmt.Println("xtray is already running.")
		return
	}

	// that.daemon.Run()

	go that.startRunnerPingServer()
	// go that.CtrlServer()
	if !that.verifier.IsRunning() {
		that.verifier.SetUseExtraOrNot(true)
		that.verifier.Run(true)
	}
	cronTime := that.conf.VerificationCron
	if !strings.HasPrefix(cronTime, "@every") {
		cronTime = "@every 2h"
	}
	that.cron.AddFunc(cronTime, func() {
		if !that.verifier.IsRunning() {
			that.verifier.Run(false, false)
		}
	})
	that.cron.Start()
	that.Restart(0)
	<-StopChan
	os.Exit(0)
}

func (that *Runner) Restart(pIdx int) (result string) {
	if that.client == nil {
		that.client = clients.NewLocalClient(clients.TypeSing)
	}
	that.client.Close()
	pxy := that.verifier.GetProxyByIndex(pIdx)
	if pxy != nil {
		that.client.SetProxy(pxy)
		that.client.SetInPortAndLogFile(that.conf.NeoBoxClientInPort, "")
		err := that.client.Start()
		if err == nil {
			result = fmt.Sprintf("%d.%s", pIdx, pxy.String())
		} else {
			result = err.Error()
		}
	}
	return
}

func (that *Runner) Exit() {
	StopChan <- struct{}{}
}