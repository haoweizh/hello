import React, {useEffect, useState} from "react";
import useWebSocket from "react-use-websocket";

const Data:React.FC = ()=>{
  const [wsData, setWsData] = useState({})


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



  useEffect(() => {
    console.log(lastJsonMessage);

    if (lastJsonMessage && Object.keys(lastJsonMessage).length > 0){
      console.log(lastJsonMessage);
      const dataArray = Object.entries(lastJsonMessage).map(([key, value]) => ({ key, ...value }));

// 按照 CreatedAt 属性排序数组
      dataArray.sort((a, b) => new Date(a.CreatedAt).getTime() - new Date(b.CreatedAt).getTime());
      console.log(dataArray);

// 从排序后的数组中提取每个条目的键值对
      const sortedData = dataArray.reduce((acc, { key, ...rest }) => {
        acc[key] = rest;
        return acc;
      }, {});
      const sortObj = new Map();

      dataArray.forEach(item=>{
        // @ts-ignore
        // sortObj[item.key] = item;
        sortObj.set(item.key, item);

      })

      // const res = Object.fromEntries(sortedData);
      console.log(sortObj);

      let tempArr = {}

      // @ts-ignore
      sortObj && sortObj.forEach((value:any, key:any) => {
        // @ts-ignore
        tempArr[key] = value;
      })

      lastJsonMessage && setWsData(tempArr)
    }
  }, [lastJsonMessage]);

  // @ts-ignore
  return (
    <div>
      <div style={{fontSize: "15px",backgroundColor:"#ccc", display: 'flex', flexWrap: "wrap", gap: "6px", padding: "8px"}}>

        {
          Object.keys(wsData).length > 0 && Object.keys(wsData).map((key, index) => {
            return (
              <div key={index} style={{display:'flex', flexDirection:"column" ,flexWrap:"wrap"}}>
                <span>{key}:{JSON.stringify({
                  Symbol: wsData[key].Symbol,
                  IntervalSeconds:wsData[key].IntervalSeconds,
                  CreatedAt:wsData[key].CreatedAt.substr(0, 19),
                })}</span>
              </div>
            )
        })}

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
