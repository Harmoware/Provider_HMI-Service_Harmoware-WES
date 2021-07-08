package synerex

import (
	"fmt"
	"log"

	sxmqtt "github.com/synerex/proto_mqtt"
	pb "github.com/synerex/synerex_api"
	sxutil "github.com/synerex/synerex_sxutil"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// unityへmqttのgo messageを送信する
func SendMQTTGomessage(id int, x, y float64) {
	tpc := fmt.Sprintf("cmd/human/%d/go", id)
	msg := fmt.Sprintf(`{
		"x":     %f,
		"y":     %f,
		"angle": %f
		}`,
		x, y, 0.0,
	)

	rcdMqtt := sxmqtt.MQTTRecord{
		Topic:  tpc,
		Time:   timestamppb.Now(),
		Record: []byte(msg),
	}

	out, _ := proto.Marshal(&rcdMqtt)
	cont := pb.Content{Entity: out}
	smo := sxutil.SupplyOpts{
		Name:  "HSIM_MQTT_Publish",
		Cdata: &cont,
	}
	_, nerr := Mqttclient.NotifySupply(&smo)
	if nerr != nil {
		log.Print("Connection failure", nerr)
	}
}
