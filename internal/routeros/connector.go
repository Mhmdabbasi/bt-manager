package routerosadapter

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	ros "github.com/Mhmdabbasi/routeros-api-Go"
)

var ErrInvalidRequest=errors.New("invalid RouterOS connection parameters")
type TestRequest struct{Address string `json:"address"`;Port int `json:"port"`;Username string `json:"username"`;Password string `json:"password"`;UseTLS bool `json:"useTls"`}
type RouterInfo struct{Identity string `json:"identity"`;Version string `json:"version"`;Model string `json:"model"`;BoardName string `json:"boardName"`;Uptime string `json:"uptime"`;CPU string `json:"cpu"`;CPUCount string `json:"cpuCount"`;FreeMemory string `json:"freeMemory"`;TotalMemory string `json:"totalMemory"`}

func TestConnection(ctx context.Context,r TestRequest)(RouterInfo,error){
	r.Address=strings.TrimSpace(r.Address);r.Username=strings.TrimSpace(r.Username)
	if r.Address==""||r.Username==""||r.Password==""||strings.ContainsAny(r.Address," /\\\r\n\t")||strings.Contains(r.Address,"://"){return RouterInfo{},ErrInvalidRequest}
	if r.Port==0{if r.UseTLS{r.Port=ros.DefaultPortTLS}else{r.Port=ros.DefaultPort}};if r.Port<1||r.Port>65535{return RouterInfo{},ErrInvalidRequest}
	timeout:=8*time.Second;if d,ok:=ctx.Deadline();ok&&time.Until(d)<timeout{timeout=time.Until(d)};if timeout<=0{return RouterInfo{},context.DeadlineExceeded}
	cfg:=ros.NewConfig().WithAddress(r.Address).WithPort(r.Port).WithCredentials(r.Username,r.Password).WithTimeout(timeout).WithReadTimeout(timeout).WithWriteTimeout(timeout).WithRetry(0, 0).WithAutoReconnect(false);if r.UseTLS{cfg.WithTLS()}
	api,err:=ros.NewWithConfig(cfg);if err!=nil{return RouterInfo{},fmt.Errorf("create RouterOS client: %w",err)};if err:=api.Connect();err!=nil{return RouterInfo{},fmt.Errorf("connect to router: %w",err)};defer api.Close()
	info,err:=api.System.GetInfoContext(ctx);if err!=nil{return RouterInfo{},fmt.Errorf("read router system information: %w",err)}
	return RouterInfo{Identity:info.Identity,Version:info.Version,Model:info.Model,BoardName:info.BoardName,Uptime:info.Uptime,CPU:info.CPU,CPUCount:info.CPUCount,FreeMemory:info.FreeMemory,TotalMemory:info.TotalMemory},nil
}
