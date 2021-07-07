package main

import (
	"flag"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/golang/protobuf/proto"

	"github.com/gorilla/websocket"
	proto_wes "github.com/synerex/proto_wes"
	api "github.com/synerex/synerex_api"
	pbase "github.com/synerex/synerex_proto"
	sxutil "github.com/synerex/synerex_sxutil"

	pick "github.com/Harmoware/Provider_HMI-Service_Harmoware-WES/picking"
	sx "github.com/Harmoware/Provider_HMI-Service_Harmoware-WES/synerex"
	ws "github.com/Harmoware/Provider_HMI-Service_Harmoware-WES/websocket"
)

var (
	nodesrv = flag.String("nodesrv", "127.0.0.1:9990", "Node ID Server")

	nosx     = flag.Bool("nosx", false, "Do not use synerex. standalone Websocket Service")
	upgrader = websocket.Upgrader{} // use default options

	//mqttclient      *sxutil.SXServiceClient
	warehouseclient *sxutil.SXServiceClient

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
	//log.Printf("Receive Message! %s , %v", sp.SupplyName, sp)

	switch sp.SupplyName {

	case "CLI_wms_order":
		rcd := &proto_wes.WmsOrder{}
		err := proto.Unmarshal(sp.Cdata.Entity, rcd)
		if err == nil {
			log.Printf("fail to load order")
		}
		newB := pick.NewBatchInfo(rcd)
		bs.AddBatch(newB)
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
			*v <- mes
		}
	}
	smu.Unlock()
}

// func sendOne(mes []byte, id int64) {
// 	smu.Lock()
// 	defer smu.Unlock()
// 	if client, ok := clientList[id]; ok {
// 		*client <- mes
// 		log.Printf("Sending to %d:%s ", id, mes)
// 	}
// }

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
			err = c.WriteMessage(mt, message[5:])
			if err != nil {
				log.Println("Echo Error!:", err)
				break
			}

		} else if strings.HasPrefix(mes, "send:") {
			// chat with other clietns
			log.Printf("Sending to others:%s ", mes[5:])
			sendAll(message[5:], &mychan)

		} else if strings.HasPrefix(mes, "cmd:") {
			//picking command
			action := mes[4:]
			if strings.HasPrefix(action, "start") {
				//start new batch
				newb := bs.AssignBatch()
				if newb == nil {
					err := c.WriteMessage(mt, []byte("no batch"))
					if err != nil {
						log.Println("Error during message writing:", err)
						break
					}
				}

			} else if strings.HasPrefix(action, "status") {
				// send status

			} else if strings.HasPrefix(action, "next") {
				// next item
				er := user.NextItem()
				if er != nil {
					err := c.WriteMessage(mt, []byte(er.Error()))
					if err != nil {
						log.Println("Error during message writing:", err)
						break
					}
				}

			} else if strings.HasPrefix(action, "robot") {
				// to do send robot information

			} else {
				err := c.WriteMessage(mt, []byte("unknown action"))
				if err != nil {
					log.Println("Error during message writing:", err)
					break
				}
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

	go ws.RunWebsocketServer(handleWebsocket) // start web socket server
	wg := sync.WaitGroup{}                    // for syncing other goroutines

	if *nosx {

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
		// argJSON1 := "{Client:HMI_SERVICE_MQTT}"
		// mqttclient = sxutil.NewSXServiceClient(client, pbase.MQTT_GATEWAY_SVC, argJSON1)
		argJSON2 := "{Client:HMI_SERVICE_WAREHOUSE}"
		warehouseclient = sxutil.NewSXServiceClient(client, pbase.WAREHOUSE_SVC, argJSON2)
		log.Print("Start Subscribe")
		go sx.SubscribeWarehouseSupply(warehouseclient, supplyWarehouseCallback)
	}

	bs = pick.NewBatchStatus()

	wg.Add(1)

	wg.Wait()

	sxutil.CallDeferFunctions() // cleanup!
}
