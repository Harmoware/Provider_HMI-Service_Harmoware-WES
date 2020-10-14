package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
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
)

var (
	nodesrv  = flag.String("nodesrv", "127.0.0.1:9990", "Node ID Server")
	wsAddr   = flag.String("wsaddr", "localhost:10090", "HMI-Service WebSocket Listening Port")
	nosx     = flag.Bool("nosx", false, "Do not use synerex. standalone Websocket Service")
	upgrader = websocket.Upgrader{} // use default options

	mqttclient      *sxutil.SXServiceClient
	warehouseclient *sxutil.SXServiceClient

	sxServerAddress string
	mu              sync.Mutex
	smu             sync.Mutex // for websocket
	clientsSend     = make([]*chan []byte, 0)

	clientMsg  = make(map[int64][]byte)
	clientList = make(map[int64]*chan []byte)
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
type humanStateJson struct {
	Currenttask  string `json:"currenttask"`
	Nextposition string `json:"nextposition"`
	Workingtime  string `json:"workingtime"`
	Movedistance string `json:"movedistance"`
	Message      string `json:"message"`
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
				Currenttask:  fmt.Sprintf("%d/%d", len(rcd.PickedItem)+1, rcd.WmsItemNum),
				Nextposition: rcd.NextItem,
				Workingtime:  fmt.Sprintf("%dsec", rcd.ElapsedTime),
				Movedistance: fmt.Sprintf("%fm", rcd.MoveDistance),
				Message:      rcd.Message,
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

func sendOne(mes []byte, id int64) {
	smu.Lock()
	defer smu.Unlock()
	if client, ok := clientList[id]; ok {
		*client <- mes
		log.Printf("Sending to %d:%s ", id, mes)
	}
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
		} else {
			noSpace := strings.Replace(mes, " ", "", -1)
			id, err := strconv.Atoi(noSpace)
			if err != nil {
				log.Print(err, mes)
			} else {
				smu.Lock()
				if _, ok := clientList[int64(id)]; !ok {
					clientList[int64(id)] = &mychan
				}
				smu.Unlock()
			}
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

	go runWebsocketServer() // start web socket server
	wg := sync.WaitGroup{}  // for syncing other goroutines

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
