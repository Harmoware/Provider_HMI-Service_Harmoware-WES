package synerex

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/synerex/synerex_api"
	sxutil "github.com/synerex/synerex_sxutil"
)

var (
	SxServerAddress string
	Mu              sync.Mutex
	Mqttclient      *sxutil.SXServiceClient
	Warehouseclient *sxutil.SXServiceClient
)

func ReconnectClient(client *sxutil.SXServiceClient) {
	Mu.Lock()
	if client.SXClient != nil {
		client.SXClient = nil
		log.Printf("Client reset \n")
	}
	Mu.Unlock()
	time.Sleep(5 * time.Second) // wait 5 seconds to reconnect
	Mu.Lock()
	if client.SXClient == nil {
		newClt := sxutil.GrpcConnectServer(SxServerAddress)
		if newClt != nil {
			log.Printf("Reconnect server [%s]\n", SxServerAddress)
			client.SXClient = newClt
		}
	} else { // someone may connect!
		log.Print("Use reconnected server\n", SxServerAddress)
	}
	Mu.Unlock()
}

func SubscribeWarehouseSupply(client *sxutil.SXServiceClient, callback func(*sxutil.SXServiceClient, *synerex_api.Supply)) {
	//wait message from CLI
	ctx := context.Background()
	for { // make it continuously working..
		client.SubscribeSupply(ctx, callback)
		log.Print("Error on subscribe WAREHOUSE")
		ReconnectClient(client)
	}
}
