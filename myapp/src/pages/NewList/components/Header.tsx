import React from "react";
import {Flex, Popconfirm, PopconfirmProps, Typography} from "antd";
import {CloseOutlined} from "@ant-design/icons";
import moment from "moment";
import {removeMonitors} from "@/services/ant-design-pro/api";

const {Title, Paragraph, Text, Link} = Typography;


interface iHeader {
  item: API.MonitorItem
  onDel?: () => void
}

const Header: React.FC<iHeader> = (props) => {


  const {item, onDel} = props;
  const handleDel = async (ID: number) => {
    const res = await removeMonitors({id: ID})
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
    <Flex justify={'space-between'}>
      <Flex wrap={'wrap'} style={{fontSize: "15px", maxWidth: "400px"}} gap={10}>
        <Flex>
          <Text style={{fontSize: "15px"}}>Symbol:</Text>
          <Text style={{fontSize: "15px"}} type={'danger'}>{item.Symbol}</Text>
        </Flex>
        <Flex>
          <Text style={{fontSize: "15px"}}>IntervalSeconds:</Text>
          <Text style={{fontSize: "15px"}} type={'danger'}>{item.IntervalSeconds}</Text>
        </Flex>
        <Flex>
          <Text style={{fontSize: "15px"}}>WarnChange:</Text>
          <Text style={{fontSize: "15px"}} type={'danger'}>{item.WarnChange}</Text>
        </Flex>
        <Flex>
          <Text style={{fontSize: "15px"}}>WarnIncrease:</Text>
          <Text style={{fontSize: "15px"}} type={'danger'}>{item.WarnIncrease}</Text>
        </Flex>
        <Flex>
          <Text style={{fontSize: "15px"}}>WarnVolume:</Text>
          <Text style={{fontSize: "15px"}} type={'danger'}>{item.WarnVolume}</Text>
        </Flex>
        <Flex>
          <Text style={{fontSize: "15px"}}>Volume24:</Text>
          <Text style={{fontSize: "15px"}} type={'danger'}>{item.Volume24}</Text>
        </Flex>
        <Flex>
          <Text style={{fontSize: "15px"}}>CreateTime:</Text>
          <Text style={{fontSize: "15px"}} type={'danger'}>{moment(item.CreatedAt).format("YYYY-MM-DD HH:mm:ss")}</Text>
        </Flex>
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
          <CloseOutlined/>
        </Popconfirm>

      </Flex>
    </Flex>
  )
}

export default Header;
