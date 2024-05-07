import React, { useEffect, useState } from 'react';
import Chart from './components/Chart/Chart';
import {Affix, Button, Flex} from "antd";
import {FormattedMessage} from "@@/exports";

const request = {
  method: 'SUBSCRIBE',
  params: ['btcusdt@trade'],
  id: 1,
};

const Monitor = () => {
  const user = localStorage.getItem("user");
  // This can also be an async getter function. See notes below on Async Urls.
  const socketUrl = 'ws://47.74.31.113:8075/monitor?address=' + user;
  const [ws, setWs] = useState<{
    [property: string]: any;
  }>({})
  const [wsData, setWsData] = useState<{
    [property: string]: any;
  }>({})
  const [trades, setTrades] = useState([]);

  useEffect(() => {
    const wsClient = new WebSocket(socketUrl);
    wsClient.onopen = () => {
      setWs(wsClient);
      wsClient.send(`hello`);
    };
    wsClient.onclose = () => console.log('ws closed');
    return () => {
      wsClient.close();
    };
  }, []);

  useEffect(() => {
    console.log("monitor render页面我就会触发")
    return () => {
      console.log("monitor unmount当前监听")
    }
  })
  useEffect(() => {
    if (ws) {
      ws.onmessage = (evt: { data: {}; }) => {
        console.log(`get ws msg`+Object.keys(evt.data));
        setWsData(Object.keys(evt.data))
      };
    }
  }, [ws]);

  return (
    <>
      <Flex vertical gap={'middle'}>
        {
          <div className="list">
            <div style={{fontSize: "15px", display: 'flex', flexWrap: "wrap", gap: "6px", padding: "8px"}}>
              {
                Object.keys(wsData).length > 0 && Object.keys(ws).map((key, index) => {
                  return (
                    <div key={index}>
                      <span>{key}:{wsData[key]}</span>
                    </div>
                  )
                })
              }
            </div>
          </div>
        }
      </Flex>
    </>
  );
};
export default Monitor;
