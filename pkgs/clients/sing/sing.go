package sing

import (
	"context"
	"fmt"
	"runtime"

	tui "github.com/moqsien/goutils/pkgs/gtui"
	log "github.com/moqsien/goutils/pkgs/logs"
	"github.com/moqsien/neobox/pkgs/iface"
	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/option"
)

/*
Sing-box client
*/
type Client struct {
	inPort  int
	proxy   iface.IProxy
	logPath string
	*box.Box
	cancel context.CancelFunc
	conf   []byte
}

func NewClient() *Client {
	return &Client{}
}

func (that *Client) SetInPortAndLogFile(inPort int, logPath string) {
	that.inPort = inPort
	that.logPath = logPath
}

func (that *Client) SetProxy(p iface.IProxy) {
	that.proxy = p
}

func (that *Client) Start() (err error) {
	that.conf = GetConfStr(that.proxy, that.inPort, that.logPath)
	if len(that.conf) > 0 {
		opt := &option.Options{}
		if err = opt.UnmarshalJSON(that.conf); err != nil {
			log.Error("[Build config for Sing-Box failed] ", err)
			return err
		}

		var ctx context.Context
		ctx, that.cancel = context.WithCancel(context.Background())
		that.Box, err = box.New(box.Options{
			Context: ctx,
			Options: *opt,
		})
		if err != nil {
			that.Close()
			log.Error("[Init Sing-Box Failed] ", err)
			return
		}

		err = that.Box.Start()
		if err != nil {
			that.Close()
			log.Error("[Start Sing-Box Failed] ", err)
			return
		}
		tui.PrintInfof("Sing-box started successfully [%s]", that.proxy.Decode())
		return
	} else {
		log.Error("[Parse config file failed]")
		return fmt.Errorf("cannot parse proxy")
	}
}

func (that *Client) cancelBox() {
	if that.cancel != nil {
		that.cancel()
	}
}

func (that *Client) Close() {
	that.conf = nil
	that.cancelBox()
	if that.Box != nil {
		that.Box.Close()
		that.Box = nil
		runtime.GC()
	}
}

func (that *Client) GetConf() []byte {
	return that.conf
}
