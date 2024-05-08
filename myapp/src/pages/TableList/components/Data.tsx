import React, {useEffect, useState} from "react";
import useWebSocket from "react-use-websocket";

const Data:React.FC = ()=>{
  const [wsData, setWsData] = useState<{
    [property: string]: any;
  }>({})


  const user = localStorage.getItem("user");
  // This can also be an async getter function. See notes below on Async Urls.
  const socketUrl = 'ws://47.74.31.113:8075/monitor?address=' + user;
  const {
    sendMessage,
    sendJsonMessage,
    lastMessage,
    lastJsonMessage,
    readyState,
    getWebSocket,
  } = useWebSocket(socketUrl, {
    onOpen: () => {
      sendMessage("ping")
    },
    //Will attempt to reconnect on all close events, such as server shutting down
    // shouldReconnect: (closeEvent) => true,
    heartbeat: {
      message: 'ping',
      returnMessage: 'pong',
      timeout: 60000, // 1 minute, if no response is received, the connection will be closed
      interval: 10000, // every 25 seconds, a ping message will be sent
    },
  });

  // // Ping every 60 second
  // const HEARTBEAT_INTERVAL = 60000;
  //
  //
  // useEffect(() => {
  //   let heartbeatInterval = undefined
  //     // Start heartbeat interval
  //      heartbeatInterval = setInterval(() => {
  //        console.log(222);
  //        sendMessage("ping");
  //     }, HEARTBEAT_INTERVAL);
  //
  //   // Clean up interval on component unmount
  //   return () => clearInterval(heartbeatInterval);
  // }, []);

  useEffect(() => {
    console.log(lastJsonMessage);
    if (lastJsonMessage && Object.keys(lastJsonMessage).length > 0){
      lastJsonMessage && setWsData(lastJsonMessage)
    }
  }, [lastJsonMessage]);

  return (
    <div>
      <div style={{fontSize: "15px",backgroundColor:"#ccc", display: 'flex', flexWrap: "wrap", gap: "6px", padding: "8px"}}>

        {
          Object.keys(wsData).length > 0 && Object.keys(wsData).map((key, index) => {
            return (
              <div key={index}>
                <span>{key}:{JSON.stringify(wsData[key])}</span>
              </div>
            )
          })
        }

        {/*<span>PriceIncrease:{data?.PriceIncrease}</span>*/}
        {/*<span>PriceChange:{data?.PriceChange}</span>*/}
        {/*<span>PriceCurrent:{data?.PriceCurrent}</span>*/}
        {/*<span>PriceStart:{data?.PriceStart}</span>*/}
        {/*<span>PriceHigh:{data?.PriceHigh}</span>*/}
        {/*<span>PriceLow:{data?.PriceLow}</span>*/}
        {/*<span>Volume:{data?.Volume}</span>*/}
        {/*<span>TimeInterval:{data?.TimeInterval}</span>*/}
        {/*<span>Start:{moment(data?.Start).format("YYYY-MM-DD HH:mm:ss")}</span>*/}
        {/*<span>End:{moment(data?.End).format("YYYY-MM-DD HH:mm:ss")}</span>*/}
        {/*<span>SlideRing:{data?.SlideRing}</span>*/}
      </div>

    </div>
  )
}


export default Data;
