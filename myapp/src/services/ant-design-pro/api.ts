// @ts-ignore
/* eslint-disable */
import { request } from '@umijs/max';
import qs from 'qs';

/** 获取当前的用户 GET /api/currentUser */
export async function currentUser(options?: { [key: string]: any }) {
  return request<API.CurrentUser>('/api/currentUser', {
    method: 'GET',
    ...(options || {}),
  });
}

/** 此处后端没有提供注释 GET /api/notices */
export async function outLogin(options?: { [key: string]: any }) {
  return request<API.NoticeIconList>('/api/notices', {
    method: 'GET',
    ...(options || {}),
  });
}

/** 此处后端没有提供注释 GET /api/notices */
export async function addRule(options?: { [key: string]: any }) {
  return request<API.NoticeIconList>('/api/add_monitor', {
    method: 'POST',
    headers: { 'Content-Type': 'multipart/form-data' },
    data:{
      data:JSON.stringify(options)
    },

  });
}
/** 此处后端没有提供注释 GET /api/notices */
export async function removeRule(options?: { [key: string]: any }) {
  return request<API.NoticeIconList>('/api/notices', {
    method: 'GET',
    ...(options || {}),
  });
}
/** 此处后端没有提供注释 GET /api/notices */
export async function rule(options?: { [key: string]: any }) {
  return request<API.NoticeIconList>('/api/get_monitors', {
    method: 'POST',
    ...(options || {}),
  });
}
/** 此处后端没有提供注释 GET /api/notices */
export async function getNotices(options?: { [key: string]: any }) {
  return request<API.NoticeIconList>('/api/notices', {
    method: 'GET',
    ...(options || {}),
  });
}
/** 此处后端没有提供注释 GET /api/notices */
export async function updateRule(options?: { [key: string]: any }) {
  return request<API.NoticeIconList>('/api/notices', {
    method: 'GET',
    ...(options || {}),
  });
}
// export async function login(options?: { [key: string]: any }) {
//   return request<API.NoticeIconList>('/api/login', {
//     method: 'POST',
//     headers: { "Content-Type": "multipart/form-data" },
//     data:{
//       user:params.mail,
//       code:params.code,
//     }
//   });
// }
//

export async function login(body: API.LoginParams, options?: { [key: string]: any }) {
  return request<API.LoginResult>('/api/login', {
    method: 'POST',
    headers: { 'Content-Type': 'multipart/form-data' },
    data: {
      code: body.captcha,
      user: body.email,
    },
    ...(options || {}),
  });
}

/** 此处后端没有提供注释 GET /api/notices */
export async function getMonitors(options?: { [key: string]: any }) {
  return request<API.MonitorList>('/api/get_monitors', {
    method: 'POST',
    ...(options || {}),
  });
}


export async function removeMonitors(options?: { id: number }) {
  return request<API.MonitorList>('/api/remove_monitor', {
    method: 'POST',
    headers: { 'Content-Type': 'multipart/form-data' },
    data:options
  });
}
