// @ts-ignore
/* eslint-disable */
import { request } from '@umijs/max';

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
  return request<API.NoticeIconList>('/api/notices', {
    method: 'GET',
    ...(options || {}),
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
  return request<API.NoticeIconList>('/api/notices', {
    method: 'GET',
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
export async function login(options?: { [key: string]: any }) {
  return request<API.NoticeIconList>('/api/login', {
    method: 'POST',
    ...(options || {}),
  });
}

/** 发送验证码1 POST /api/login/captcha */
export async function getFakeCaptcha(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.getFakeCaptchaParams,
  options?: { [key: string]: any },
) {
  return request<API.FakeCaptcha>('/api/pw', {
    method: 'POST',
    params: {
      ...params,
    },
    // ...(options || {}),
  });
}

/** 此处后端没有提供注释 GET /api/notices */
export async function getMonitors(options?: { [key: string]: any }) {
  return request<API.NoticeIconList>('/api/get_monitors', {
    method: 'GET',
    ...(options || {}),
  });
}
