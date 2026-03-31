const BASE = '/api'

export interface BotInfo {
  client_id: string
  state: string
  user_id?: string
}

export interface ChatMessage {
  id: string
  bot_id: string
  from: string
  to: string
  text: string
  direction: 'in' | 'out'
  has_image: boolean
  image_b64?: string
  has_file: boolean
  file_name?: string
  file_b64?: string
  file_size?: number
  timestamp: number
}

export interface ActivateResponse {
  state: string
  qr_url?: string
}

export async function listBots(): Promise<BotInfo[]> {
  const res = await fetch(`${BASE}/bots`)
  return res.json()
}

export async function activateBot(id: string): Promise<ActivateResponse> {
  const res = await fetch(`${BASE}/bots/${id}/activate`, { method: 'POST' })
  return res.json()
}

export async function deactivateBot(id: string): Promise<void> {
  await fetch(`${BASE}/bots/${id}/deactivate`, { method: 'POST' })
}

export async function getBotState(id: string): Promise<{ state: string }> {
  const res = await fetch(`${BASE}/bots/${id}/state`)
  return res.json()
}

export async function getMessages(botId: string): Promise<ChatMessage[]> {
  const res = await fetch(`${BASE}/bots/${botId}/messages`)
  return res.json()
}

export async function sendText(botId: string, to: string, text: string): Promise<void> {
  await fetch(`${BASE}/bots/${botId}/send`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ to, text }),
  })
}

export async function sendImage(botId: string, to: string, imageB64: string, caption: string): Promise<void> {
  await fetch(`${BASE}/bots/${botId}/send-image`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ to, image_b64: imageB64, caption }),
  })
}

export async function sendFile(botId: string, to: string, fileB64: string, fileName: string, caption: string): Promise<void> {
  await fetch(`${BASE}/bots/${botId}/send-file`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ to, file_b64: fileB64, file_name: fileName, caption }),
  })
}

export async function sendTyping(botId: string, to: string, cancel = false): Promise<void> {
  await fetch(`${BASE}/bots/${botId}/typing`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ to, cancel }),
  })
}
