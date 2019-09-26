package daemons

import (
	"fmt"
	"github.com/bilibili/kratos/pkg/log"
	"github.com/gorilla/websocket"
	"github.com/mariuspass/recws"
	"os"
	"os/signal"
	"subscan-end/libs/substrate"
	"subscan-end/utiles"
	"sync"
	"syscall"
	"time"
)

var (
	lockId         sync.Mutex
	substrateRpcId int
	SubscribeConn  *recws.RecConn
	subscribeOnce  sync.Once
)

func Subscribe() {
	var err error
	fmt.Println("start ....", utiles.ProviderEndPoint)
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT)
	SubscribeConn = &recws.RecConn{KeepAliveTimeout: 10 * time.Second}
	SubscribeConn.Dial(utiles.ProviderEndPoint, nil)

	defer SubscribeConn.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if !SubscribeConn.IsConnected() {
				continue
			}
			_, message, err := SubscribeConn.ReadMessage()
			if err != nil {
				log.Error("read:", err)
				return
			}
			log.Info("recv: %s", message)
			parserDistribution(message, srv)
		}
	}()

	ticker := time.NewTicker(time.Second * 3)
	defer ticker.Stop()
	substrateRpcId = 1
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			lockId.Lock()
			substrateRpcId++
			err := SubscribeConn.WriteMessage(websocket.TextMessage, substrate.SystemHealth(substrateRpcId))
			if substrateRpcId >= 100 {
				substrateRpcId = 2
			}
			lockId.Unlock()
			if err != nil {
				log.Info("write:", err)
			} else {
				setHeartBeat("substrate")
			}
		case <-interrupt:
			log.Info("interrupt")
			err := SubscribeConn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Error("write close:", err)
				return
			}
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			return
		default:
			if !SubscribeConn.IsConnected() {
				continue
			}
			subscribeOnce.Do(func() {
				if err = SubscribeConn.WriteMessage(websocket.TextMessage, substrate.ChainGetRuntimeVersion(1)); err != nil {
					log.Info("write:", err)
				}
				if err = SubscribeConn.WriteMessage(websocket.TextMessage, substrate.ChainSubscribeNewHead(101)); err != nil {
					log.Info("write:", err)
				}
			})
		}
	}

}
