// @ts-nocheck
import React, {useCallback, useEffect,useMemo, useState} from "react";

import useWebSocket from 'react-use-websocket';

import { Manager } from "socket.io-client";
import {abs} from "stylis";
import moment from "moment";
import {Tag} from "antd";


interface iWsData {
  item: API.MonitorItem
}


export interface WsResDataProps {
  SlideRing: SlideRing
  Start: string
  End: string
  TimeInterval: string
  PriceHigh: string
  PriceLow: string
  Volume: number
  PriceStart: string
  PriceCurrent: string
  PriceIncrease: number
  PriceChange: number
}

export interface SlideRing {
}


const WsData: React.FC<iWsData> = (props) => {

  const {item} = props;
  const [data, setData] = useState<WsResDataProps>()




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
            {
              readyState ?   <Tag color="success">连接</Tag>:  <Tag color="error">断开</Tag>
            }
            <div style={{fontSize:"15px", display:'flex', flexWrap:"wrap", gap:"6px", padding:"8px"}}>

              {
                Object.keys(data).map((key, index) => {
                  return (
                    <div key={index}>
                      <span>{key}:{data[key]}</span>
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
          </>
        )
      }

    </div>
  )
}

export default WsData;
