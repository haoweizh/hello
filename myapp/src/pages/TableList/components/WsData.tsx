// @ts-nocheck
import React, {useMemo, useState} from "react";

import {io} from "socket.io-client";
import {abs} from "stylis";
import moment from "moment";


interface iWsData {
  item: API.MonitorItem
}


export interface WsResDataProps {
  SlideRing: SlideRing
  Start: string
  End: string
  TimeInterval: number
  PriceHigh: number
  PriceLow: number
  Volume: number
  PriceStart: number
  PriceCurrent: number
  PriceIncrease: number
  PriceChange: number
}

export interface SlideRing {
}


const WsData: React.FC<iWsData> = (props) => {

  const {item} = props;
  const [data, setData] = useState<WsResDataProps>()

  const ws = useMemo(() => {
    let ws = new WebSocket("ws://47.74.31.113:8075/monitor?id=" + item.ID);
    ws.onopen = function (evt) {
      console.log("Connection open ...");
      ws.send("Hello WebSockets!");
    };

    ws.onmessage = function (evt) {
      console.log("Received Message: " + evt.data);
      setData(JSON.parse(evt.data))
      ws.close();
    };

    ws.onclose = function (evt) {
      console.log("Connection closed.");
    };

  }, [item.ID]);


  const isHighLight = useMemo(() => {
    if (
      (data?.PriceChange > item.WarnChange) ||
      (data?.PriceIncrease > item.WarnIncrease && data?.PriceIncrease > 0) ||
      (abs(data?.PriceIncrease) > item.WarnIncrease && data?.PriceIncrease < 0) ||
      (data?.Volume > item.WarnVolume)) {
      return true;
    } else {
      return false
    }
  }, [item.ID, data])

  return (
    <div style={{backgroundColor: isHighLight? "green":""}}>
      {
        data && (
          <>
            <div style={{fontSize:"12px", display:'flex', flexWrap:"wrap", gap:"6px", padding:"8px"}}>
            <span>PriceIncrease:{data?.PriceIncrease}</span>
            <span>PriceChange:{data?.PriceChange}</span>
            <span>PriceCurrent:{data?.PriceCurrent}</span>
            <span>PriceStart:{data?.PriceStart}</span>
            <span>PriceHigh:{data?.PriceHigh}</span>
            <span>PriceLow:{data?.PriceLow}</span>
            <span>Volume:{data?.Volume}</span>
            <span>TimeInterval:{data?.TimeInterval}</span>
            <span>Start:{moment(data?.Start).format("YYYY-MM-DD HH:mm:ss")}</span>
            <span>End:{moment(data?.End).format("YYYY-MM-DD HH:mm:ss")}</span>
            {/*<span>SlideRing:{data?.SlideRing}</span>*/}
            </div>
          </>
        )
      }

    </div>
  )
}

export default WsData;
