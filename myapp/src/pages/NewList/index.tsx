import {addRule, getMonitors, removeRule, rule, updateRule} from '@/services/ant-design-pro/api';
import {PlusOutlined} from '@ant-design/icons';
import type {ActionType, ProColumns, ProDescriptionsItemProps} from '@ant-design/pro-components';
import {
  FooterToolbar,
  ModalForm,
  PageContainer,
  ProDescriptions,
  ProFormText,
  ProFormTextArea,
  ProTable,
} from '@ant-design/pro-components';
import {FormattedMessage, useIntl} from '@umijs/max';
import {Button, Drawer, Flex, Form, Input, List, Typography, message, Affix,} from 'antd';
import React, {useEffect, useRef, useState} from 'react';
import type {FormValueType} from './components/UpdateForm';
import UpdateForm from './components/UpdateForm';
import ConfigPage from './components/WsData';
import {useRequest} from "@@/plugin-request";
import WsData from "./components/WsData";
import Header from "@/pages/TableList/components/Header";
import "./index.less"
import useWebSocket from "react-use-websocket";
import {WsResDataProps} from "@/pages/TableList/components/WsData";

/**
 * @en-US Add node
 * @zh-CN 添加节点
 * @param fields
 */
const handleAdd = async (fields: API.RuleListItem) => {
  const hide = message.loading('正在添加');
  try {
    const res = await addRule({...fields});
    if (res.status == "ok"){
      hide();
      message.success('Added successfully');
      return true;
    }else {
      message.error('Adding failed'+res.msg);
    }
  } catch (error) {
    hide();
    message.error('Adding failed, please try again!');
    return false;
  }
};

/**
 * @en-US Update node
 * @zh-CN 更新节点
 *
 * @param fields
 */
const handleUpdate = async (fields: FormValueType) => {
  const hide = message.loading('Configuring');
  try {
    await updateRule({
      name: fields.name,
      desc: fields.desc,
      key: fields.key,
    });
    hide();

    message.success('Configuration is successful');
    return true;
  } catch (error) {
    hide();
    message.error('Configuration failed, please try again!');
    return false;
  }
};

/**
 *  Delete node
 * @zh-CN 删除节点
 *
 * @param selectedRows
 */
const handleRemove = async (selectedRows: API.RuleListItem[]) => {
  const hide = message.loading('正在删除');
  if (!selectedRows) return true;
  try {
    await removeRule({
      key: selectedRows.map((row) => row.key),
    });
    hide();
    message.success('Deleted successfully and will refresh soon');
    return true;
  } catch (error) {
    hide();
    message.error('Delete failed, please try again');
    return false;
  }
};

const TableList: React.FC = () => {
  /**
   * @en-US Pop-up window of new window
   * @zh-CN 新建窗口的弹窗
   *  */
  const [createModalOpen, handleModalOpen] = useState<boolean>(false);
  /**
   * @en-US The pop-up window of the distribution update window
   * @zh-CN 分布更新窗口的弹窗
   * */
  const [updateModalOpen, handleUpdateModalOpen] = useState<boolean>(false);

  const [showDetail, setShowDetail] = useState<boolean>(false);

  const actionRef = useRef<ActionType>();
  const [currentRow, setCurrentRow] = useState<API.RuleListItem>();
  const [selectedRowsState, setSelectedRows] = useState<API.RuleListItem[]>([]);

  /**
   * @en-US International configuration
   * @zh-CN 国际化配置
   * */
  const intl = useIntl();

  const [top, setTop] = React.useState<number>(60);
  const [bottom, setBottom] = React.useState<number>(100);

  const {data, error, loading, refresh} = useRequest<API.MonitorListResp>(getMonitors);

  const [wsData, setWsData] = useState<{
    [property: string]: any;
  }>({})


  const [form] = Form.useForm();
  // console.log(data?.monitors);

  const monitors = data?.monitors;
  // This can also be an async getter function. See notes below on Async Urls.
  const socketUrl = 'ws://47.74.31.113:8073/monitor?address=57059329@qq.com';

  const {
    sendMessage,
    sendJsonMessage,
    lastMessage,
    lastJsonMessage,
    readyState,
    getWebSocket,
  } = useWebSocket(socketUrl, {
    onOpen: () => {
      sendMessage("hello")
    },
    //Will attempt to reconnect on all close events, such as server shutting down
    shouldReconnect: (closeEvent) => true,
    // heartbeat: {
    //   message: 'ping',
    //   returnMessage: 'pong',
    //   timeout: 60000, // 1 minute, if no response is received, the connection will be closed
    //   interval: 10000, // every 25 seconds, a ping message will be sent
    // },
    // heartbeat:true
  });

  // Ping every 60 second
  const HEARTBEAT_INTERVAL = 60000;

  useEffect(() => {
    // Start heartbeat interval
    const heartbeatInterval = setInterval(() => {
      sendMessage("ping");
    }, HEARTBEAT_INTERVAL);

    // Clean up interval on component unmount
    return () => clearInterval(heartbeatInterval);
  }, [sendJsonMessage]);
  useEffect(() => {
    if (lastJsonMessage){
      console.log(Object.keys(lastJsonMessage));
    }
    lastJsonMessage && setWsData(lastJsonMessage)
  }, [lastJsonMessage]);


  useEffect(() => {

    if (createModalOpen){
      form.resetFields();
    }

  }, [createModalOpen]);

  return (
    <>
      <Affix offsetTop={top}>
        <Button type="primary" onClick={()=>{
          handleModalOpen(true)
          form.resetFields()
        }}>
          Add Config
        </Button>
      </Affix>
      <Flex vertical gap={'middle'}>
        {
              <div className="list">
                {/*<List*/}
                {/*  header={<Header item={item} onDel={()=>refresh()} />}*/}
                {/*  footer={null}*/}
                {/*  size={"small"}*/}
                {/*  bordered*/}
                {/*>*/}
                {/*  <WsData item={item}/>*/}
                {/*</List>*/}
                <div style={{fontSize: "15px", display: 'flex', flexWrap: "wrap", gap: "6px", padding: "8px"}}>

                  {
                    Object.keys(wsData).length > 0 && Object.keys(wsData).map((key, index) => {
                      return (
                        <div key={index}>
                          <span>{key}:{wsData[key]}</span>
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
        }
      </Flex>
      <ModalForm
        title={intl.formatMessage({
          id: 'pages.searchTable.createForm.newRule',
          defaultMessage: 'New rule',
        })}
        width="400px"
        open={createModalOpen}
        onOpenChange={handleModalOpen}
        form={form}

        onFinish={async (value) => {
          const success = await handleAdd(value as API.RuleListItem);
          if (success) {
            handleModalOpen(false);
            refresh();
            if (actionRef.current) {
              actionRef.current.reload();
            }
          }
        }}
      >
        <ProFormText
          rules={[
            {
              required: true,
              message: (
                <FormattedMessage
                  id="pages.searchTable.ruleName"
                  defaultMessage="币种 is required"
                />
              ),
            },
          ]}
          placeholder={'请输入币种'}
          width="md"
          name="Symbol"
        />
        <ProFormText
          rules={[
            {
              required: true,
              message: (
                <FormattedMessage
                  id="pages.searchTable.ruleName"
                  defaultMessage="IntervalSeconds is required"
                />
              ),
            },
          ]}
          placeholder={'请输入监控时间段(秒)'}
          width="md"
          name="IntervalSeconds"
        />
        {/*<ProFormText*/}
        {/*  rules={[*/}
        {/*    {*/}
        {/*      required: true,*/}
        {/*      message: (*/}
        {/*        <FormattedMessage*/}
        {/*          id="pages.searchTable.ruleName"*/}
        {/*          defaultMessage="Market name is required"*/}
        {/*        />*/}
        {/*      ),*/}
        {/*    },*/}
        {/*  ]}*/}
        {/*  width="md"*/}
        {/*  placeholder={'请输入Market'}*/}
        {/*  name="Market"*/}
        {/*/>*/}
        <ProFormText
          rules={[
            {
              required: true,
              message: (
                <FormattedMessage
                  id="pages.searchTable.ruleName"
                  defaultMessage="振幅is required"
                />
              ),
            },
          ]}
          width="md"
          placeholder={'请输入预警振幅'}
          name="WarnChange"
        />
        <ProFormText
          rules={[
            {
              required: true,
              message: (
                <FormattedMessage
                  id="pages.searchTable.ruleName"
                  defaultMessage="涨幅is required"
                />
              ),
            },
          ]}
          width="md"
          placeholder={'请输入预警涨幅'}
          name="WarnIncrease"
        />
        <ProFormText
          rules={[
            {
              required: true,
              message: (
                <FormattedMessage
                  id="pages.searchTable.ruleName"
                  defaultMessage="成交量 is required"
                />
              ),
            },
          ]}
          width="md"
          placeholder={'请输入预警成交量'}
          name="WarnVolume"
        />
      </ModalForm>
    </>
  );
};

export default TableList;
