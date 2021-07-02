package main

import (
	"context"
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

	pick "github.com/Harmoware/Provider_HMI-Service_Harmoware-WES/picking"
	ws "github.com/Harmoware/Provider_HMI-Service_Harmoware-WES/websocket"
)

var (
	nodesrv = flag.String("nodesrv", "127.0.0.1:9990", "Node ID Server")

	nosx     = flag.Bool("nosx", false, "Do not use synerex. standalone Websocket Service")
	upgrader = websocket.Upgrader{} // use default options

	mqttclient      *sxutil.SXServiceClient
	warehouseclient *sxutil.SXServiceClient

	sxServerAddress string
	mu              sync.Mutex
	smu             sync.Mutex // for websocket

	clientsSend = make([]*chan []byte, 0)

	clientMsg  = make(map[int64][]byte)
	clientList = make(map[int64]*chan []byte) // id と websocket-clientの 1対1対応

	userList = make(map[int64]*pick.WorkerInfo)
)

func reconnectClient(client *sxutil.SXServiceClient) {
	mu.Lock()
	if client.SXClient != nil {
		client.SXClient = nil
		log.Printf("Client reset \n")
	}
	mu.Unlock()
	time.Sleep(5 * time.Second) // wait 5 seconds to reconnect
	mu.Lock()
	if client.SXClient == nil {
		newClt := sxutil.GrpcConnectServer(sxServerAddress)
		if newClt != nil {
			log.Printf("Reconnect server [%s]\n", sxServerAddress)
			client.SXClient = newClt
		}
	} else { // someone may connect!
		log.Print("Use reconnected server\n", sxServerAddress)
	}
	mu.Unlock()
}

func subscribeWarehouseSupply(client *sxutil.SXServiceClient) {
	//wait message from CLI
	ctx := context.Background()
	for { // make it continuously working..
		client.SubscribeSupply(ctx, supplyWarehouseCallback)
		log.Print("Error on subscribe WAREHOUSE")
		reconnectClient(client)
	}
}

//hololens message
type pos2 struct {
	X float32 `json:"x"`
	Y float32 `json:"y"`
}
type humanStateJson struct {
	Currenttask     string `json:"currenttask"`
	Nextposition    string `json:"nextposition"`
	Workingtime     string `json:"workingtime"`
	Movedistance    string `json:"movedistance"`
	Message         string `json:"message"`
	Currentposition pos2   `json:"currentposition"`
	Targetposition  pos2   `json:"targetposition"`
}

func supplyWarehouseCallback(clt *sxutil.SXServiceClient, sp *api.Supply) {
	if sp.SenderId == uint64(clt.ClientID) {
		// ignore my message.
		return
	}
	//log.Printf("Receive Message! %s , %v", sp.SupplyName, sp)

	switch sp.SupplyName {

	case "WES_amr_state_publish":
		rcd := &proto_wes.AmrState{}
		err := proto.Unmarshal(sp.Cdata.Entity, rcd)
		if err == nil {

		}

	case "WES_state_publish":
		rcd := &proto_wes.WesState{}
		err := proto.Unmarshal(sp.Cdata.Entity, rcd)
		if err == nil {

		}

	case "WES_human_state_publish":
		rcd := &proto_wes.WesHumanState{}
		err := proto.Unmarshal(sp.Cdata.Entity, rcd)
		if err == nil {
			humanState := humanStateJson{
				Currenttask:     fmt.Sprintf("%d/%d", len(rcd.PickedItem)+1, rcd.WmsItemNum),
				Nextposition:    rcd.NextItem,
				Workingtime:     fmt.Sprintf("%dsec", rcd.ElapsedTime),
				Movedistance:    fmt.Sprintf("%fm", rcd.MoveDistance),
				Message:         rcd.Message,
				Currentposition: pos2{X: rcd.LatestPos.X, Y: rcd.LatestPos.Y},
				Targetposition:  pos2{X: rcd.TargetPos.X, Y: rcd.LatestPos.Y},
			}
			msg, jerr := json.Marshal(humanState)
			if jerr != nil {
				log.Print(jerr)
				break
			} else {
				sendOne(msg, rcd.Id)
			}
		}

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
			//			log.Printf("Not send myself %d", i)
			continue
		} else {
			*v <- mes
		}
	}
	smu.Unlock()
}

func sendOne(mes []byte, id int64) {
	smu.Lock()
	defer smu.Unlock()
	if client, ok := clientList[id]; ok {
		*client <- mes
		log.Printf("Sending to %d:%s ", id, mes)
	}
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

	defer c.Close() // until closed
	for {
		mt, message, err := c.ReadMessage()
		if err != nil {
			log.Println("Read Error!:", err)
			break
		}
		mes := string(message)
		log.Printf("Receive: %s", mes)

		if strings.HasPrefix(mes, "echo:") {
			err = c.WriteMessage(mt, message[5:])
			if err != nil {
				log.Println("Echo Error!:", err)
				break
			}
		} else if strings.HasPrefix(mes, "send:") {
			// we need to send other clients!
			log.Printf("Sending to others:%s ", mes[5:])
			sendAll(message[5:], &mychan)

		} else if strings.HasPrefix(mes, "cmd:") {
			//do actions
			action := mes[4:]
			if strings.HasPrefix(action, "status") {
				//send status

			} else if strings.HasPrefix(action, "next") {

			}

		} else if strings.HasPrefix(mes, "id:") {
			// for first subscribe
			//noSpace := strings.Replace(mes, " ", "", -1)
			//id, err = strconv.Atoi(noSpace)
			id, err = strconv.Atoi(mes[3:])
			if err != nil {
				log.Print(err, mes)
			} else {
				smu.Lock()
				clientList[int64(id)] = &mychan
				smu.Unlock()
			}
		}
	}
	smu.Lock()

	for i, v := range clientsSend {
		if v == &mychan {
			clientsSend = append(clientsSend[:i], clientsSend[i+1:]...)
			close(mychan)
			if id != -1 {
				delete(clientList, int64(id))
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
		sxServerAddress = srv
		client := sxutil.GrpcConnectServer(srv)
		argJSON1 := fmt.Sprintf("{Client:HMI_SERVICE_MQTT}")
		mqttclient = sxutil.NewSXServiceClient(client, pbase.MQTT_GATEWAY_SVC, argJSON1)
		argJSON2 := fmt.Sprintf("{Client:HMI_SERVICE_WAREHOUSE}")
		warehouseclient = sxutil.NewSXServiceClient(client, pbase.WAREHOUSE_SVC, argJSON2)
		log.Print("Start Subscribe")
		go subscribeWarehouseSupply(warehouseclient)
	}
	wg.Add(1)

	wg.Wait()

	sxutil.CallDeferFunctions() // cleanup!
}
