export const WEBHOOK_PROVIDERS = [
  {value: 'generic', label: '通用 Webhook'},
  {value: 'dingtalk', label: '钉钉机器人'},
  {value: 'feishu', label: '飞书机器人'}
]

export const WEBHOOK_TEMPLATES = {
  generic: `{
  "task_id": "{{.TaskId}}",
  "task_name": "{{.TaskName}}",
  "status": "{{.Status}}",
  "result": "{{.Result}}",
  "remark": "{{.Remark}}"
}`,
  dingtalk: `{
  "msgtype": "text",
  "text": {
    "content": "任务：{{.TaskName}}\\n状态：{{.Status}}\\n结果：{{.Result}}"
  }
}`,
  feishu: `{
  "msg_type": "text",
  "content": {
    "text": "任务：{{.TaskName}}\\n状态：{{.Status}}\\n结果：{{.Result}}"
  }
}`
}

export function webhookPayload (form, hasUsableSecret) {
  if (!WEBHOOK_PROVIDERS.some(item => item.value === form.provider)) throw new Error('请选择有效的 Webhook 平台')
  const secret = String(form.secret || '').trim()
  if (secret.length > 512 || /[\r\n]/.test(secret) || secret.includes('\u0000')) throw new Error('签名密钥格式无效')
  if (form.sign_enabled && form.provider === 'generic') throw new Error('加签请选择钉钉或飞书平台')
  if (form.sign_enabled && ((!secret && !hasUsableSecret) || form.clear_secret)) throw new Error('开启加签后请填写签名密钥')
  return {...form, secret, sign_enabled: form.sign_enabled ? 1 : 0, clear_secret: form.clear_secret ? 1 : 0}
}
