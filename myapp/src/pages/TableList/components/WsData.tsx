import React from "react";

import { io } from "socket.io-client";



interface iWsData  {
  id:number
}
const WsData:React.FC<iWsData> = (props)=>{

  const {id} = props;
  const socket = io("ws://47.74.31.113:8075/monitor",{
    query:{
      id:id
    },

  });
  socket.on("connect", () => {
    console.log("connect");
  });


  return (
    <div>
      hello
    </div>
  )
}

export default WsData;
