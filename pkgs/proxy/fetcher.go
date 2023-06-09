package proxy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/gocolly/colly/v2"
	crypt "github.com/moqsien/goutils/pkgs/crypt"
	"github.com/moqsien/goutils/pkgs/gutils"
	log "github.com/moqsien/goutils/pkgs/logs"
	"github.com/moqsien/neobox/pkgs/conf"
	"github.com/moqsien/neobox/pkgs/parser"
)

type RawList struct {
	Total int      `json:"total"`
	List  []string `json:"list"`
}

type RawResult struct {
	VmessList  *RawList `json:"vmess"`
	SSRList    *RawList `json:"ssr"`
	VlessList  *RawList `json:"vless"`
	SSList     *RawList `json:"ss"`
	Trojan     *RawList `json:"trojan"`
	Other      *RawList `json:"other"`
	UpdateTime string   `json:"update_time"`
}

/*
Download raw proxies list.
*/
type Fetcher struct {
	collector  *colly.Collector
	Conf       *conf.NeoBoxConf
	RawProxies *RawResult
	key        *conf.RawListEncryptKey
	path       string
}

func NewFetcher(c *conf.NeoBoxConf) *Fetcher {
	log.SetLogger(c.NeoLogFileDir)
	return &Fetcher{
		collector: colly.NewCollector(),
		Conf:      c,
		RawProxies: &RawResult{
			VmessList: &RawList{List: []string{}},
			VlessList: &RawList{List: []string{}},
			SSRList:   &RawList{List: []string{}},
			SSList:    &RawList{List: []string{}},
			Trojan:    &RawList{List: []string{}},
			Other:     &RawList{List: []string{}},
		},
		key:  conf.NewEncryptKey(),
		path: filepath.Join(c.NeoWorkDir, c.RawUriFileName),
	}
}

func (that *Fetcher) DownloadFile() (success bool) {
	that.collector.OnResponse(func(r *colly.Response) {
		dCrypt := crypt.NewCrptWithKey([]byte(that.key.Get()))
		if result, err := dCrypt.AesDecrypt(r.Body); err == nil {
			os.WriteFile(that.path, result, os.ModePerm)
			success = true
		} else {
			log.Error("[Parse rawFile failed] ", err)
		}
	})
	that.collector.Visit(that.Conf.RawUriURL)
	that.collector.Wait()
	return
}

func (that *Fetcher) GetRawProxies(force ...bool) *RawResult {
	flag := false
	if len(force) > 0 {
		flag = force[0]
	}
	if ok, _ := gutils.PathIsExist(that.path); !ok || flag {
		flag = that.DownloadFile()
	} else {
		flag = true
	}
	if flag {
		if rawProxy, err := os.ReadFile(that.path); err == nil {
			json.Unmarshal(rawProxy, that.RawProxies)
		} else {
			log.Error("parse rawlist file failed. ", err.Error(), "AES_KEY: ", that.key.Get())
			return nil
		}
	}
	return that.RawProxies
}

func (that *Fetcher) readExtaVPNList() (r []string) {
	if ok, _ := gutils.PathIsExist(that.Conf.ExtraVPNsDir); !ok {
		os.MkdirAll(that.Conf.ExtraVPNsDir, 0666)
		return
	}
	dList, _ := os.ReadDir(that.Conf.ExtraVPNsDir)
	for _, d := range dList {
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".txt") {
			if content, err := os.ReadFile(filepath.Join(that.Conf.ExtraVPNsDir, d.Name())); err == nil && len(content) > 0 {
				for _, s := range strings.Split(string(content), "\n") {
					s = strings.Trim(strings.TrimSpace(s), "\r")
					if parser.ParseScheme(s) != "" {
						r = append(r, s)
					}
				}
			}
		}
	}
	return
}

func (that *Fetcher) GetRawProxyList(force ...bool) (r []string) {
	result := that.GetRawProxies(force...)
	if result == nil {
		return
	}
	r = append(r, result.VmessList.List...)
	r = append(r, result.VlessList.List...)
	r = append(r, result.Trojan.List...)
	r = append(r, result.SSList.List...)
	r = append(r, result.SSRList.List...)
	r = append(r, that.readExtaVPNList()...)
	return
}

type RawStatistics struct {
	Vmess  int `json:"vmess"`
	SSR    int `json:"ssr"`
	Vless  int `json:"vless"`
	SS     int `json:"ss"`
	Trojan int `json:"trojan"`
	Other  int `json:"other"`
}

func (that *Fetcher) GetStatistics() *RawStatistics {
	result := that.GetRawProxies()
	if result == nil {
		return &RawStatistics{}
	}
	return &RawStatistics{
		Vmess:  len(result.VmessList.List),
		Vless:  len(result.VlessList.List),
		SSR:    len(result.SSRList.List),
		SS:     len(result.SSList.List),
		Trojan: len(result.Trojan.List),
		Other:  len(result.Other.List),
	}
}
