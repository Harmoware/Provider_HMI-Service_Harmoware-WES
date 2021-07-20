package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"

	"github.com/gorilla/websocket"
	proto_wes "github.com/synerex/proto_wes"
	api "github.com/synerex/synerex_api"
	pbase "github.com/synerex/synerex_proto"
	sxutil "github.com/synerex/synerex_sxutil"

	"github.com/Harmoware/Provider_HMI-Service_Harmoware-WES/logging"
	pick "github.com/Harmoware/Provider_HMI-Service_Harmoware-WES/picking"
	sx "github.com/Harmoware/Provider_HMI-Service_Harmoware-WES/synerex"
	ws "github.com/Harmoware/Provider_HMI-Service_Harmoware-WES/websocket"
)

var (
	nodesrv  = flag.String("nodesrv", "127.0.0.1:9990", "Node ID Server")
	nosx     = flag.Bool("nosx", false, "Do not use synerex. standalone Websocket Service")
	loggingf = flag.Bool("log", false, "logging in log/datefile")

	upgrader = websocket.Upgrader{} // use default options

	smu sync.Mutex // for websocket

	clientsSend = make([]*chan []byte, 0)
	clientList  = make(map[int64]*chan []byte) // id と websocket-clientの 1対1対応

	userList = make(map[int64]*pick.WorkerInfo) // id と user

	bs *pick.BatchStatus
)

func supplyWarehouseCallback(clt *sxutil.SXServiceClient, sp *api.Supply) {
	if sp.SenderId == uint64(clt.ClientID) {
		// ignore my message.
		return
	}

	switch sp.SupplyName {

	case "CLI_wms_order":
		rcd := &proto_wes.WmsOrder{}
		err := proto.Unmarshal(sp.Cdata.Entity, rcd)
		if err == nil {
			log.Printf("proto: fail to unmarshal order")
		}
		newB := bs.NewBatchInfo(rcd)
		bs.AddBatch(newB)
		log.Print("add new batch")
	}
}

// send message to websocket
func handleSender(c *websocket.Conn, mychan *chan []byte) {
	for {
		mes, ok := <-*mychan
		if !ok {
			break // channel was closed!
		}
		c.WriteMessage(websocket.TextMessage, mes)
	}
}

// send all
func sendAll(mes []byte, mychan *chan []byte) {
	smu.Lock()
	for _, v := range clientsSend {
		if v == mychan {
			//log.Printf("Not send myself %d", i)
			continue
		} else {
			out := fmt.Sprintf(`{"type":"send", "payload":%s}`, string(mes))
			*v <- []byte(out)
		}
	}
	smu.Unlock()
}

func sendWebSocketMsg(c *websocket.Conn, mt int, msg []byte, typ string) error {
	out := fmt.Sprintf(`{"type":"%s", "payload":%s}`, typ, string(msg))
	err := c.WriteMessage(mt, []byte(out))
	if err != nil {
		log.Println("Error during message writing:", err)
		return err
	}
	return nil
}

func handleWebsocket(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("Upgrade Error!:", err)
		return
	}
	id := -1

	// we need to remove chan if wsocket is closed.
	mychan := make(chan []byte) //for sending message
	log.Printf("Starting socket :%v", mychan)
	smu.Lock()
	clientsSend = append(clientsSend, &mychan)
	smu.Unlock()
	go handleSender(c, &mychan)
	var user *pick.WorkerInfo

	defer c.Close() // until closed
	for {
		mt, message, err := c.ReadMessage()
		if err != nil {
			log.Println("Read Error!:", err)
			break
		}
		mes := string(message)
		log.Printf("Receive: %s", mes)

		// repeat message
		if strings.HasPrefix(mes, "echo:") {
			err := sendWebSocketMsg(c, mt, message[5:], "echo")
			if err != nil {
				continue
			}

		} else if strings.HasPrefix(mes, "send:") {
			// chat with other clietns
			log.Printf("Sending to others:%s ", mes[5:])
			sendAll(message[5:], &mychan)

		} else if strings.HasPrefix(mes, "cmd:") {
			if user == nil {
				sendWebSocketMsg(c, mt, []byte("please set id before command"), "error")
				continue
			}
			//picking command
			action := mes[4:]
			if strings.HasPrefix(action, "start") {
				//start new batch
				ok := user.OKBatch()
				if ok != nil {
					err_mes := string("\"" + ok.Error() + "\"")
					sendWebSocketMsg(c, mt, []byte(err_mes), "error")
					continue
				}
				newb, err := bs.AssignBatch()
				if err != nil {
					err_mes := string("\"" + err.Error() + "\"")
					sendWebSocketMsg(c, mt, []byte(err_mes), "error")
					continue
				}
				user.SetBatch(newb)
				if !*nosx {
					sx.SendMQTTGomessage(id, user.CurrentItem.Pos.X, user.CurrentItem.Pos.Y)
				}
				out, e := json.Marshal(user.CurrentItem)
				if e != nil {
					log.Println(e)
					continue
				}
				sendWebSocketMsg(c, mt, out, "item")

			} else if strings.HasPrefix(action, "status") {
				// send status
				out, e := json.Marshal(user.CurrentBatch)
				if e != nil {
					log.Println(e)
					continue
				}
				sendWebSocketMsg(c, mt, out, "batch")

			} else if strings.HasPrefix(action, "next") {
				// next item
				next, er := user.NextItem()
				if er != nil {
					err_mes := string("\"" + er.Error() + "\"")
					sendWebSocketMsg(c, mt, []byte(err_mes), "error")
				}
				if !*nosx {
					sx.SendMQTTGomessage(id, user.CurrentItem.Pos.X, user.CurrentItem.Pos.Y)
				}
				out, e := json.Marshal(next)
				if e != nil {
					log.Println(e)
					continue
				}
				sendWebSocketMsg(c, mt, out, "item")
			} else if strings.HasPrefix(action, "finish") {
				er := user.FinishBatch()
				if er != nil {
					err_mes := string("\"" + er.Error() + "\"")
					sendWebSocketMsg(c, mt, []byte(err_mes), "error")
					continue
				}
				sendWebSocketMsg(c, mt, []byte("finish"), "finish")

				// } else if strings.HasPrefix(action, "route") {
				// 	var x, y float64
				// 	_, err := fmt.Sscanf(action, "route %f,%f", &x, &y)
				// 	if err != nil {
				// 		err := c.WriteMessage(mt, []byte("format error: cmd:route <x>,<y>"))
				// 		if err != nil {
				// 			log.Println("Error during message writing:", err)
				// 			continue
				// 		}
				// 	}
				// 	if user.CurrentItem == nil {
				// 		err := c.WriteMessage(mt, []byte("routing: you are not working yet"))
				// 		if err != nil {
				// 			log.Println("Error during message writing:", err)
				// 			continue
				// 		}
				// 	}
				// 	w, err := pick.GetPath(x, y, user.CurrentItem)
				// 	if err != nil {
				// 		err := c.WriteMessage(mt, []byte(err.Error()))
				// 		if err != nil {
				// 			log.Println("Error during message writing:", err)
				// 			continue
				// 		}
				// 	}
				// 	out, e := json.Marshal(w)
				// 	if e != nil {
				// 		log.Println(e)
				// 		continue
				// 	}
				// 	err = c.WriteMessage(mt, out)
				// 	if err != nil {
				// 		log.Println("Error during message writing:", err)
				// 		continue
				// 	}

			} else if strings.HasPrefix(action, "robot") {
				// to do send robot information
				sendWebSocketMsg(c, mt, []byte("test_robot0:00,00"), "robot")

			} else {
				err_mes := string("\"" + "unknown action" + "\"")
				sendWebSocketMsg(c, mt, []byte(err_mes), "error")
			}

		} else if strings.HasPrefix(mes, "id:") {
			id, err = strconv.Atoi(mes[3:])
			if err != nil {
				log.Print(err, mes)
			} else {
				smu.Lock()
				clientList[int64(id)] = &mychan
				if _, ok := userList[int64(id)]; !ok {
					userList[int64(id)] = pick.NewWorkerInfo(int64(id))
				}
				user = userList[int64(id)]
				smu.Unlock()
				user.Connect()
			}
		}
	}

	// disconnect
	smu.Lock()
	for i, v := range clientsSend {
		if v == &mychan {
			clientsSend = append(clientsSend[:i], clientsSend[i+1:]...)
			close(mychan)
			if id != -1 {
				delete(clientList, int64(id))
				userList[int64(id)].DisConnect()
			}
			log.Printf("Close channel %v", mychan)
			break
		}
	}
	smu.Unlock()

}

func main() {
	log.Printf("HMI-Service(%s) built %s sha1 %s", sxutil.GitVer, sxutil.BuildTime, sxutil.Sha1Ver)
	flag.Parse()
	go sxutil.HandleSigInt() //exit by Ctrl + C

	wg := sync.WaitGroup{} // for syncing other goroutines

	bs = pick.NewBatchStatus()

	//logging configuration
	if *loggingf {
		now := time.Now()
		logging.LoggingSettings("log/" + now.Format("2006-01-02") + "/" + now.Format("2006-01-02-15") + ".log")
	}

	if *nosx {
		bs.ReadOrder("assets/wms_order_demo.csv")
	} else {
		sxutil.RegisterDeferFunction(sxutil.UnRegisterNode)
		channelTypes := []uint32{pbase.WAREHOUSE_SVC, pbase.MQTT_GATEWAY_SVC}
		// obtain synerex server address from nodeserv
		srv, err := sxutil.RegisterNode(*nodesrv, "HMI-service", channelTypes, nil)
		if err != nil {
			log.Fatal("Can't register node...")
		}
		log.Printf("Connecting Server [%s]\n", srv)
		sx.SxServerAddress = srv
		client := sxutil.GrpcConnectServer(srv)
		argJSON1 := "{Client:HMI_SERVICE_MQTT}"
		sx.Mqttclient = sxutil.NewSXServiceClient(client, pbase.MQTT_GATEWAY_SVC, argJSON1)
		argJSON2 := "{Client:HMI_SERVICE_WAREHOUSE}"
		sx.Warehouseclient = sxutil.NewSXServiceClient(client, pbase.WAREHOUSE_SVC, argJSON2)
		log.Print("Start Subscribe")
		go sx.SubscribeWarehouseSupply(sx.Warehouseclient, supplyWarehouseCallback)
	}
	go ws.RunWebsocketServer(handleWebsocket) // start web socket server

	wg.Add(1)

	wg.Wait()

	sxutil.CallDeferFunctions() // cleanup!
}
