package websocket

import (
	"flag"
	"html/template"
	"log"
	"net/http"
)

var (
	WsAddr = flag.String("wsaddr", "localhost:10090", "HMI-Service WebSocket Listening Port")
)

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

func RunWebsocketServer(handler func(http.ResponseWriter, *http.Request)) {
	log.Printf("Starting Websocket server on %s", *WsAddr)

	http.HandleFunc("/", home)
	http.HandleFunc("/w", handler)

	err := http.ListenAndServe(*WsAddr, nil)

	log.Printf("Websocket listening Error!", err)
}
