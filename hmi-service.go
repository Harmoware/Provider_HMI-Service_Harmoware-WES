package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"

	"github.com/gorilla/websocket"
	sxmqtt "github.com/synerex/proto_mqtt"
	proto_wes "github.com/synerex/proto_wes"
	api "github.com/synerex/synerex_api"
	pb "github.com/synerex/synerex_api"
	pbase "github.com/synerex/synerex_proto"
	sxutil "github.com/synerex/synerex_sxutil"
)

var (
	nodesrv = flag.String("nodesrv", "127.0.0.1:9990", "Node ID Server")
	noPrint = flag.Bool("noPrint", false, "do not display publish msg")
	wsAddr  = flag.String("wsaddr", "localhost:10090", "HMI-Service WebSocket Listening Port")

	upgrader = websocket.Upgrader{} // use default options

	mqttclient      *sxutil.SXServiceClient
	warehouseclient *sxutil.SXServiceClient

	sxServerAddress string
	mu              sync.Mutex
	smu             sync.Mutex // for websocket
	clientsSend     = make([]*chan []byte, 0)
)

func init() {
}

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

func subscribeMqttSupply(client *sxutil.SXServiceClient) {
	//wait message from CLI
	ctx := context.Background()
	for { // make it continuously working..
		client.SubscribeSupply(ctx, supplyMqttCallback)
		log.Print("Error on subscribe WAREHOUSE")
		reconnectClient(client)
	}
}

func supplyMqttCallback(clt *sxutil.SXServiceClient, sp *api.Supply) {
	if sp.SenderId == uint64(clt.ClientID) {
		// ignore my message.
		return
	}

	rcd := &sxmqtt.MQTTRecord{}
	err := proto.Unmarshal(sp.Cdata.Entity, rcd)
	if err == nil {
		if rcd.Topic == "cmd/simulator/set_state" {
			log.Printf("get set state and reset")
			mu.Lock()
			//			ioserv.BroadcastToAll("set_state", "")
			time.Sleep(time.Second) //リセットのために止める
			mu.Unlock()
		}
	}

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

//-- publish するJSONメッセージ定義 --//
type position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type hsimHumanPublishMsgJson struct {
	Id            int64    `json:"id"`
	Pos           position `json:"pos"`
	TargetPos     position `json:"targetPos"`
	Wms           int64    `json:"wmsID"`
	RestWms       []int64  `json:"restWmsID"`
	StartUnix     int64    `json:"startUnix"`
	ElapsedTime   int64    `json:"elapsedTime"`
	Status        string   `json:"status"`
	PickedItemNum int64    `json:"pickedNum"`
	ItemNum       int64    `json:"itemNum"`
	PickedItem    []string `json:"pickedItem"`
	RestItem      []string `json:"restItem"`
}

type robotPublishMsgJson struct {
	Id          int64    `json:"id"`
	Pos         position `json:"pos"`
	TargetPos   position `json:"targetPos"`
	Wms         int64    `json:"wmsID"`
	StartUnix   int64    `json:"startUnix"`
	ElapsedTime int64    `json:"elapsedTime"`
	Status      string   `json:"status"`
	Target      string   `json:"target"`
}

type cartPublishMsgJson struct {
	Id            int64    `json:"id"`
	Pos           position `json:"pos"`
	HumanID       int64    `json:"humanID"`
	ElapsedTime   int64    `json:"elapsedTime"`
	LatestMsgUnix int64    `json:"latestMsgUnix"`
	Status        string   `json:"status"`
	AssignAmr     int64    `json:"assignAmr"`
}

type wesPublishMsgJson struct {
	SimType     string  `json:"simType"`
	WorkingWms  []int64 `json:"workingWms"`
	RestWms     []int64 `json:"restWms"`
	FinishWms   []int64 `json:"finishWms"`
	WmsGetUnix  int64   `json:"wmsGetUnix"`
	ElapsedTime int64   `json:"elapsedTime"`
	Speed       float64 `json:"speed"`
}

type hsimPublishMsgJson struct {
	SimType     string  `json:"simType"`
	IdList      []int64 `json:"idList"`
	WorkingWms  []int64 `json:"workingWms"`
	FinishWms   []int64 `json:"finishWms"`
	WmsGetUnix  int64   `json:"wmsGetUnix"`
	ElapsedTime int64   `json:"elapsedTime"`
}

type wesHumanPublishMsgJson struct {
	Id            int64    `json:"id"`
	PickedItem    []string `json:"pickedItem"`
	NoPickedItem  []string `json:"noPickedItem"`
	LatestMsgUnix int64    `json:"latestMsgUnix"`
	LatestPos     position `json:"latestPos"`
	Progress      float32  `json:"progress"`
	WorkingWms    int64    `json:"workingWms"`
	CartID        int64    `json:"cartID"`
	ElapsedTime   int64    `json:"elapsedTime"`
	RestWms       []int64  `json:"restWms"`
	LastItem      string   `json:"lastItem"`
}

type hsimCartPublishMsgJson struct {
	Id             int64    `json:"id"`
	HumanID        int64    `json:"humanID"`
	Status         string   `json:"status"`
	ItemList       []string `json:"itemList"`
	Pos            position `json:"position"`
	LastUpdateUnix int64    `json:"lastUpdateUnix"`
	PutUnix        int64    `json:"putUnix"`
}

func supplyWarehouseCallback(clt *sxutil.SXServiceClient, sp *api.Supply) {
	if sp.SenderId == uint64(clt.ClientID) {
		// ignore my message.
		return
	}
	log.Printf("Receive Message! %s , %v", sp.SupplyName, sp)

	switch sp.SupplyName {
	case "HSIM_human_state_publish":
		rcd := &proto_wes.HumanState{}
		err := proto.Unmarshal(sp.Cdata.Entity, rcd)
		if err == nil {
			topic := fmt.Sprintf("evt/wes/hsim/human/%d/status", rcd.Id)
			humanPub := hsimHumanPublishMsgJson{
				Id: rcd.Id,
				Pos: position{
					X: float64(rcd.Pos.X),
					Y: float64(rcd.Pos.Y),
				},
				TargetPos: position{
					X: float64(rcd.TargetPos.X),
					Y: float64(rcd.TargetPos.Y),
				},
				Wms:           rcd.WmsBatNum,
				RestWms:       rcd.RestWms,
				StartUnix:     rcd.StartTime,
				ElapsedTime:   rcd.ElapsedTime,
				Status:        rcd.State,
				PickedItemNum: rcd.PickedNum,
				ItemNum:       rcd.WmsItemNum,
				PickedItem:    rcd.PickedItem,
				RestItem:      rcd.RestItem,
			}
			message, jerr := json.Marshal(humanPub)
			if jerr != nil {
				log.Print("json Marshal failure", jerr)
			} else {
				rcd := sxmqtt.MQTTRecord{
					Topic:  topic,
					Record: message,
				}

				out, _ := proto.Marshal(&rcd)
				cont := pb.Content{Entity: out}
				smo := sxutil.SupplyOpts{
					Name:  "HumanState_Publish",
					Cdata: &cont,
				}

				_, nerr := mqttclient.NotifySupply(&smo)
				if nerr != nil { // connection failuer with current client
					log.Print("Connection failure", nerr)
				} else {
					if !*noPrint {
						log.Printf("send message %s:%s", topic, string(message))
					}
				}
			}

			mu.Lock()
			//			ioserv.BroadcastToAll("hsim-human", string(message))
			sendAll(message, nil)
			mu.Unlock()
		} else {
			log.Print("parse hsim publish failure", err)
		}
	case "WES_amr_state_publish":
		rcd := &proto_wes.AmrState{}
		err := proto.Unmarshal(sp.Cdata.Entity, rcd)
		if err == nil {
			topic := fmt.Sprintf("evt/wes/robot/%d/status", rcd.Id)
			amrPub := robotPublishMsgJson{
				Id:          rcd.Id,
				Pos:         position{X: float64(rcd.Pos.X), Y: float64(rcd.Pos.Y)},
				TargetPos:   position{X: float64(rcd.TargetPos.X), Y: float64(rcd.TargetPos.Y)},
				Wms:         rcd.WmsBatNum,
				StartUnix:   rcd.StartTime,
				ElapsedTime: rcd.ElapsedTime,
				Status:      rcd.State,
				Target:      rcd.Target,
			}
			message, jerr := json.Marshal(amrPub)
			if jerr != nil {
				log.Print("json Marshal failure", jerr)
			} else {
				rcd := sxmqtt.MQTTRecord{
					Topic:  topic,
					Record: message,
				}

				out, _ := proto.Marshal(&rcd)
				cont := pb.Content{Entity: out}
				smo := sxutil.SupplyOpts{
					Name:  "RobotState_Publish",
					Cdata: &cont,
				}

				_, nerr := mqttclient.NotifySupply(&smo)
				if nerr != nil { // connection failuer with current client
					log.Print("Connection failure", nerr)
				} else {
					if !*noPrint {
						log.Printf("send message %s:%s", topic, string(message))
					}
				}
			}
			mu.Lock()
			//			ioserv.BroadcastToAll("amr", string(message))
			mu.Unlock()
		} else {
			log.Print("parse wes-amr publish failure", err)
		}
	case "WES_cart_publish":
		rcd := &proto_wes.WesCartState{}
		err := proto.Unmarshal(sp.Cdata.Entity, rcd)
		if err == nil {
			topic := fmt.Sprintf("evt/wes/cart/%d/status", rcd.Id)
			cartPub := cartPublishMsgJson{
				Id:            rcd.Id,
				Pos:           position{X: float64(rcd.Pos.X), Y: float64(rcd.Pos.Y)},
				HumanID:       rcd.HumanID,
				ElapsedTime:   rcd.ElapsedTime,
				LatestMsgUnix: rcd.LatestMsgTime,
				Status:        rcd.Status,
				AssignAmr:     rcd.AmrID,
			}
			message, jerr := json.Marshal(cartPub)
			if jerr != nil {
				log.Print("json Marshal failure", jerr)
			} else {
				rcd := sxmqtt.MQTTRecord{
					Topic:  topic,
					Record: message,
				}

				out, _ := proto.Marshal(&rcd)
				cont := pb.Content{Entity: out}
				smo := sxutil.SupplyOpts{
					Name:  "CartState_Publish",
					Cdata: &cont,
				}

				_, nerr := mqttclient.NotifySupply(&smo)
				if nerr != nil { // connection failuer with current client
					log.Print("Connection failure", nerr)
				} else {
					if !*noPrint {
						log.Printf("send message %s:%s", topic, string(message))
					}
				}

				mu.Lock()
				//				ioserv.BroadcastToAll("cart", string(message))
				mu.Unlock()
			}

		} else {
			log.Print("parse wes-cart publish failure", err)
		}
	case "WES_state_publish":
		rcd := &proto_wes.WesState{}
		err := proto.Unmarshal(sp.Cdata.Entity, rcd)
		if err == nil {
			topic := "evt/wes/status"
			wesPub := wesPublishMsgJson{
				SimType:     rcd.SimType,
				WorkingWms:  rcd.WorkingWms,
				RestWms:     rcd.RestWms,
				FinishWms:   rcd.FinishWms,
				WmsGetUnix:  rcd.WmsGetTime,
				ElapsedTime: rcd.ElapsedTime,
				Speed:       float64(rcd.Speed),
			}
			message, jerr := json.Marshal(wesPub)
			if jerr != nil {
				log.Print("json Marshal failure", jerr)
			} else {
				rcd := sxmqtt.MQTTRecord{
					Topic:  topic,
					Record: message,
				}

				out, _ := proto.Marshal(&rcd)
				cont := pb.Content{Entity: out}
				smo := sxutil.SupplyOpts{
					Name:  "WesState_Publish",
					Cdata: &cont,
				}

				_, nerr := mqttclient.NotifySupply(&smo)
				if nerr != nil { // connection failuer with current client
					log.Print("Connection failure", nerr)
				} else {
					if !*noPrint {
						log.Printf("send message %s:%s", topic, string(message))
					}
				}
			}
			mu.Lock()
			//			ioserv.BroadcastToAll("wes", string(message))
			mu.Unlock()
		} else {
			log.Print("parse wes-cart publish failure", err)
		}

	case "HSIM_state_publish":
		rcd := &proto_wes.HsimState{}
		err := proto.Unmarshal(sp.Cdata.Entity, rcd)
		if err == nil {
			topic := "evt/wes/hsim/status"
			wesPub := hsimPublishMsgJson{
				SimType:     rcd.SimType,
				IdList:      rcd.IdList,
				WorkingWms:  rcd.WorkingWms,
				FinishWms:   rcd.FinishWms,
				WmsGetUnix:  rcd.WmsGetTime,
				ElapsedTime: rcd.ElapsedTime,
			}
			message, jerr := json.Marshal(wesPub)
			if jerr != nil {
				log.Print("json Marshal failure", jerr)
			} else {
				rcd := sxmqtt.MQTTRecord{
					Topic:  topic,
					Record: message,
				}

				out, _ := proto.Marshal(&rcd)
				cont := pb.Content{Entity: out}
				smo := sxutil.SupplyOpts{
					Name:  "HsimState_Publish",
					Cdata: &cont,
				}

				_, nerr := mqttclient.NotifySupply(&smo)
				if nerr != nil { // connection failuer with current client
					log.Print("Connection failure", nerr)
				} else {
					if !*noPrint {
						log.Printf("send message %s:%s", topic, string(message))
					}
				}
			}
			mu.Lock()
			//			ioserv.BroadcastToAll("hsim", string(message))
			mu.Unlock()
		} else {
			log.Print("parse hsim publish failure", err)
		}
	case "WES_human_state_publish":
		rcd := &proto_wes.WesHumanState{}
		err := proto.Unmarshal(sp.Cdata.Entity, rcd)
		if err == nil {
			topic := fmt.Sprintf("evt/wes/human/%d/status", rcd.Id)
			wesPub := wesHumanPublishMsgJson{
				Id:            rcd.Id,
				PickedItem:    rcd.PickedItem,
				NoPickedItem:  rcd.NoPickedItem,
				LatestMsgUnix: rcd.LatestMsgTime,
				LatestPos:     position{X: float64(rcd.LatestPos.X), Y: float64(rcd.LatestPos.Y)},
				Progress:      rcd.Progress,
				WorkingWms:    rcd.WorkingWms,
				CartID:        rcd.CartId,
				ElapsedTime:   rcd.ElapsedTime,
				RestWms:       rcd.RestWms,
				LastItem:      rcd.LastItem,
			}
			message, jerr := json.Marshal(wesPub)
			if jerr != nil {
				log.Print("json Marshal failure", jerr)
			} else {
				rcd := sxmqtt.MQTTRecord{
					Topic:  topic,
					Record: message,
				}

				out, _ := proto.Marshal(&rcd)
				cont := pb.Content{Entity: out}
				smo := sxutil.SupplyOpts{
					Name:  "WesHumanState_Publish",
					Cdata: &cont,
				}

				_, nerr := mqttclient.NotifySupply(&smo)
				if nerr != nil { // connection failuer with current client
					log.Print("Connection failure", nerr)
				} else {
					if !*noPrint {
						log.Printf("send message %s:%s", topic, string(message))
					}
				}
			}
			mu.Lock()
			//			ioserv.BroadcastToAll("human", string(message))
			mu.Unlock()
		} else {
			log.Print("parse hsim publish failure", err)
		}
	case "HSIM_cart_state_publish":
		rcd := &proto_wes.HsimCartState{}
		err := proto.Unmarshal(sp.Cdata.Entity, rcd)
		if err == nil {
			topic := fmt.Sprintf("evt/wes/hsim/cart/%d/status", rcd.Id)
			cartPub := hsimCartPublishMsgJson{
				Id:             rcd.Id,
				Pos:            position{X: float64(rcd.Pos.X), Y: float64(rcd.Pos.Y)},
				HumanID:        rcd.HumanID,
				ItemList:       rcd.Items,
				Status:         rcd.State,
				LastUpdateUnix: rcd.LastUpdateUnix,
				PutUnix:        rcd.PutUnix,
			}
			message, jerr := json.Marshal(cartPub)
			if jerr != nil {
				log.Print("json Marshal failure", jerr)
			} else {
				rcd := sxmqtt.MQTTRecord{
					Topic:  topic,
					Record: message,
				}

				out, _ := proto.Marshal(&rcd)
				cont := pb.Content{Entity: out}
				smo := sxutil.SupplyOpts{
					Name:  "HsimCartState_Publish",
					Cdata: &cont,
				}

				_, nerr := mqttclient.NotifySupply(&smo)
				if nerr != nil { // connection failuer with current client
					log.Print("Connection failure", nerr)
				} else {
					if !*noPrint {
						log.Printf("send message %s:%s", topic, string(message))
					}
				}
			}
			mu.Lock()
			//			ioserv.BroadcastToAll("hsim-cart", string(message))
			mu.Unlock()
		} else {
			log.Print("parse hsim publish failure", err)
		}
	}
}

var rootTemplate = template.Must(template.New("").Parse(`
<!DOCTYPE html>
<html>
<head>
<title> HMI-Service WebSocket test page </title>
<meta charset="utf-8">
<script>  
window.addEventListener("load", function(evt) {
    var output = document.getElementById("output");
    var input = document.getElementById("input");
    var ws;
    var print = function(message) {
        var d = document.createElement("div");
        d.textContent = message;
        output.appendChild(d);
    };
    document.getElementById("open").onclick = function(evt) {
        if (ws) {
            return false;
        }
        ws = new WebSocket("{{.}}");
        ws.onopen = function(evt) {
            print("OPEN");
        }
        ws.onclose = function(evt) {
            print("CLOSE");
            ws = null;
        }
        ws.onmessage = function(evt) {
            print("RECEIVE: " + evt.data);
        }
        ws.onerror = function(evt) {
            print("ERROR: " + evt.data);
        }
        return false;
    };
    document.getElementById("send").onclick = function(evt) {
        if (!ws) {
            return false;
        }
        print("SEND: " + input.value);
        ws.send(input.value);
        return false;
    };
    document.getElementById("close").onclick = function(evt) {
        if (!ws) {
            return false;
        }
        ws.close();
        return false;
	};
	document.getElementById("clear").onclick = function(evt){
		while(output.firstChild){
			output.removeChild(output.firstChild)
		}
	}
});
</script>
</head>
<body>
<table>
<tr><td valign="top" width="50%">
<p>Click "Open" to create a connection to the server, 
"Send" to send a message to the server and "Close" to close the connection. 

<p> For sending message: "send:<message>" for sending other nodes.
<p> "echo:" for echo test.

<form>
<button id="open">Open</button>
<button id="close">Close</button>
<p><input id="input" type="text" value="send:Hello world!" size="80">
<button id="send">Send</button>
</form>
<button id="clear">Clear</button>
</td><td valign="top" width="50%">
<div id="output"></div>
</td></tr></table>
</body>
</html>
`))

func home(w http.ResponseWriter, r *http.Request) {
	rootTemplate.Execute(w, "ws://"+r.Host+"/w")
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

func handleSender(c *websocket.Conn, mychan *chan []byte) {
	for {
		mes, ok := <-*mychan
		if !ok {
			break // channel was closed!
		}
		c.WriteMessage(websocket.TextMessage, mes)
	}
}

func handleWebsocket(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("Upgrade Error!:", err)
		return
	}
	// we need to remove chan if wsocket is closed.
	mychan := make(chan []byte)
	log.Printf("Starting socket :%v", w)
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
		}
	}
	smu.Lock()
	for i, v := range clientsSend {
		if v == &mychan {
			clientsSend = append(clientsSend[:i], clientsSend[i+1:]...)
			close(mychan)
			log.Printf("Close channel %v", mychan)
			break
		}
	}
	smu.Unlock()

}

func runWebsocketServer() {
	log.Printf("Starting Websocket server on %s", *wsAddr)

	http.HandleFunc("/", home)
	http.HandleFunc("/w", handleWebsocket)

	err := http.ListenAndServe(*wsAddr, nil)

	log.Printf("Websocket listening Error!", err)
}

func main() {
	log.Printf("HMI-Service(%s) built %s sha1 %s", sxutil.GitVer, sxutil.BuildTime, sxutil.Sha1Ver)
	flag.Parse()
	go sxutil.HandleSigInt() //exit by Ctrl + C
	sxutil.RegisterDeferFunction(sxutil.UnRegisterNode)

	channelTypes := []uint32{pbase.WAREHOUSE_SVC, pbase.MQTT_GATEWAY_SVC}
	// obtain synerex server address from nodeserv
	srv, err := sxutil.RegisterNode(*nodesrv, "State-Publish", channelTypes, nil)
	if err != nil {
		log.Fatal("Can't register node...")
	}
	log.Printf("Connecting Server [%s]\n", srv)

	wg := sync.WaitGroup{} // for syncing other goroutines

	go runWebsocketServer() // start web socket server

	sxServerAddress = srv
	client := sxutil.GrpcConnectServer(srv)
	argJSON1 := fmt.Sprintf("{Client:STATE_PUBLISH_MQTT}")
	mqttclient = sxutil.NewSXServiceClient(client, pbase.MQTT_GATEWAY_SVC, argJSON1)
	argJSON2 := fmt.Sprintf("{Client:STATE_PUBLISH_WAREHOUSE}")
	warehouseclient = sxutil.NewSXServiceClient(client, pbase.WAREHOUSE_SVC, argJSON2)

	wg.Add(1)
	log.Print("Start Subscribe")
	go subscribeWarehouseSupply(warehouseclient)
	go subscribeMqttSupply(mqttclient)

	wg.Wait()

	sxutil.CallDeferFunctions() // cleanup!
}
