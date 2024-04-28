import React from "react";

import { io } from "socket.io-client";



interface iWsData  {
  id:number
}
const WsData:React.FC<iWsData> = (props)=>{

  const {id} = props;
  let ws = new WebSocket("ws://47.74.31.113:8075/monitor?id="+id);
  ws.onopen = function(evt) {
    console.log("Connection open ...");
    ws.send("Hello WebSockets!");
  };

  ws.onmessage = function(evt) {
    console.log( "Received Message: " + evt.data);
    ws.close();
  };

  ws.onclose = function(evt) {
    console.log("Connection closed.");
  };

  return (
    <div>
      hello
    </div>
  )
}

export default WsData;
