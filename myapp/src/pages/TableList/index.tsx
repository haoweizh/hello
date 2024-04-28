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

/**
 * @en-US Add node
 * @zh-CN 添加节点
 * @param fields
 */
const handleAdd = async (fields: API.RuleListItem) => {
  const hide = message.loading('正在添加');
  try {
    await addRule({...fields});
    hide();
    message.success('Added successfully');
    return true;
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

  const [form] = Form.useForm();
  // console.log(data?.monitors);

  const monitors = data?.monitors;
  console.log(monitors);

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
          monitors && monitors.map((item: API.MonitorItem) => {

            return (
              <div className="list">
                <List
                  header={<Header item={item} />}
                  footer={null}
                  size={"small"}
                  bordered
                >
                  <WsData id={item.ID}/>
                </List>
              </div>
            )
          })
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
                  defaultMessage="Symbol is required"
                />
              ),
            },
          ]}
          placeholder={'请输入Symbol'}
          width="md"
          name="symbol"
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
          placeholder={'请输入IntervalSeconds'}
          width="md"
          name="intervalSeconds"
        />
        <ProFormText
          rules={[
            {
              required: true,
              message: (
                <FormattedMessage
                  id="pages.searchTable.ruleName"
                  defaultMessage="Market name is required"
                />
              ),
            },
          ]}
          width="md"
          placeholder={'请输入Market'}
          name="market"
        />
        <ProFormText
          rules={[
            {
              required: true,
              message: (
                <FormattedMessage
                  id="pages.searchTable.ruleName"
                  defaultMessage="WarnChange name is required"
                />
              ),
            },
          ]}
          width="md"
          placeholder={'请输入WarnChange'}
          name="warnChange"
        />
        <ProFormText
          rules={[
            {
              required: true,
              message: (
                <FormattedMessage
                  id="pages.searchTable.ruleName"
                  defaultMessage="WarnIncrease name is required"
                />
              ),
            },
          ]}
          width="md"
          placeholder={'请输入WarnIncrease'}
          name="WarnIncrease"
        />
        <ProFormText
          rules={[
            {
              required: true,
              message: (
                <FormattedMessage
                  id="pages.searchTable.ruleName"
                  defaultMessage="WarnVolume name is required"
                />
              ),
            },
          ]}
          width="md"
          placeholder={'请输入WarnVolume'}
          name="WarnVolume"
        />
      </ModalForm>
    </>
  );
};

export default TableList;
