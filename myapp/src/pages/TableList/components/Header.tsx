import React from "react";
import {Flex, message, Popconfirm, PopconfirmProps, Typography} from "antd";
import { CloseOutlined } from "@ant-design/icons";
import moment from "moment";
import {getMonitors, removeMonitors} from "@/services/ant-design-pro/api";
import {useRequest} from "@@/plugin-request";

const { Title, Paragraph, Text, Link } = Typography;



interface iHeader  {
  item: API.MonitorItem
  onDel?: () => void
}
const Header:React.FC<iHeader> = (props)=>{



  const {item,onDel} = props;
  const handleDel = async (ID: number) => {
   const res =  await removeMonitors({id:ID})
  };
  const confirm: PopconfirmProps['onConfirm'] = (e) => {
    handleDel(item.ID);
    if (onDel) {
      onDel();
    }
  };

  const cancel: PopconfirmProps['onCancel'] = (e) => {
    // console.log(e);
    // message.error('Click on No');
  };

  return (
    <Flex justify={'space-between'} >
      <Flex justify={"space-around"}  style={{fontSize: "11px", maxWidth:"400px"}} gap={10}>
       <div>
         <Flex >
           <Text style={{fontSize:"11px"}}>Symbol:</Text>
           <Text  style={{fontSize:"11px"}} type={'danger'}>{item.Symbol}</Text>
         </Flex>
         <Flex >
           <Text  style={{fontSize:"11px"}} >Market:</Text>
           <Text  style={{fontSize:"11px"}} type={'danger'}>{item.Market}</Text>
         </Flex>
         <Flex >
           <Text  style={{fontSize:"11px"}}>IntervalSeconds:</Text>
           <Text  style={{fontSize:"11px"}} type={'danger'}>{item.IntervalSeconds}</Text>
         </Flex>
         <Flex>
           <Text  style={{fontSize:"11px"}}>WarnChange:</Text>
           <Text  style={{fontSize:"11px"}} type={'danger'}>{item.WarnChange}</Text>
         </Flex>
       </div>

        <div>

          <Flex >
            <Text  style={{fontSize:"11px"}}>WarnIncrease:</Text>
            <Text   style={{fontSize:"11px"}} type={'danger'}>{item.WarnIncrease}</Text>
          </Flex>
          <Flex >
            <Text  style={{fontSize:"11px"}} >WarnVolume:</Text>
            <Text  style={{fontSize:"11px"}} type={'danger'}>{item.WarnVolume}</Text>
          </Flex>
          <Flex >
            <Text  style={{fontSize:"11px"}}>CreateTime:</Text>
            <Text  style={{fontSize:"11px"}} type={'danger'}>{moment(item.CreatedAt).format("YYYY-MM-DD HH:mm:ss")}</Text>
          </Flex>
        </div>


      </Flex>

      <Flex>
        <Popconfirm
          title="Delete the config?"
          description="Are you sure to delete this config?"
          onConfirm={confirm}
          onCancel={cancel}
          okText="Yes"
          cancelText="No"
        >
          <CloseOutlined />
        </Popconfirm>

      </Flex>
    </Flex>
  )
}

export default Header;
