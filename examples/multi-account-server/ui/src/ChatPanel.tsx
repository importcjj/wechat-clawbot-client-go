import { useEffect, useState, useRef, useCallback } from 'react'
import { Input, Button, Space, Upload, Image, Typography, Tooltip } from 'antd'
import { SendOutlined, PaperClipOutlined, EditOutlined, FileOutlined } from '@ant-design/icons'
import { getMessages, sendText, sendImage, sendFile, sendTyping, type ChatMessage } from './api'

const { Text } = Typography

interface Props {
  botId: string
  userId: string
}

function isImageFile(name: string): boolean {
  return /\.(jpg|jpeg|png|gif|webp|bmp|svg)$/i.test(name)
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

export default function ChatPanel({ botId, userId }: Props) {
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [input, setInput] = useState('')
  const [sending, setSending] = useState(false)
  const [typing, setTyping] = useState(false)
  const bottomRef = useRef<HTMLDivElement>(null)
  const typingTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const refresh = useCallback(async () => {
    const data = await getMessages(botId)
    setMessages(data)
  }, [botId])

  useEffect(() => {
    refresh()
    const t = setInterval(refresh, 2000)
    return () => clearInterval(t)
  }, [refresh])

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  const handleInputChange = (val: string) => {
    setInput(val)
    if (!typing && val) {
      setTyping(true)
      sendTyping(botId, userId).catch(() => {})
    }
    if (typingTimerRef.current) clearTimeout(typingTimerRef.current)
    typingTimerRef.current = setTimeout(() => {
      if (typing) {
        setTyping(false)
        sendTyping(botId, userId, true).catch(() => {})
      }
    }, 5000)
  }

  const handleSend = async () => {
    const text = input.trim()
    if (!text) return
    setInput('')
    setSending(true)
    if (typing) {
      setTyping(false)
      sendTyping(botId, userId, true).catch(() => {})
    }
    try {
      await sendText(botId, userId, text)
      await refresh()
    } finally {
      setSending(false)
    }
  }

  const handleFileUpload = async (file: File) => {
    const reader = new FileReader()
    reader.onload = async () => {
      const b64 = (reader.result as string).split(',')[1]
      setSending(true)
      try {
        if (isImageFile(file.name)) {
          await sendImage(botId, userId, b64, '')
        } else {
          await sendFile(botId, userId, b64, file.name, '')
        }
        await refresh()
      } finally {
        setSending(false)
      }
    }
    reader.readAsDataURL(file)
    return false
  }

  const formatTime = (ts: number) => {
    const d = new Date(ts)
    return d.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit' })
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
      {/* Header */}
      <div style={{ padding: '12px 20px', borderBottom: '1px solid #f0f0f0', background: '#fff' }}>
        <Text strong>{userId}</Text>
      </div>

      {/* Messages */}
      <div style={{ flex: 1, overflow: 'auto', padding: '16px 20px' }}>
        {messages.map(msg => (
          <div
            key={msg.id}
            style={{
              display: 'flex',
              justifyContent: msg.direction === 'out' ? 'flex-end' : 'flex-start',
              marginBottom: 12,
            }}
          >
            <div
              style={{
                maxWidth: '70%',
                padding: '8px 12px',
                borderRadius: 8,
                background: msg.direction === 'out' ? '#1677ff' : '#fff',
                color: msg.direction === 'out' ? '#fff' : '#333',
                boxShadow: '0 1px 2px rgba(0,0,0,0.08)',
              }}
            >
              {/* Image */}
              {msg.has_image && msg.image_b64 && (
                <div style={{ marginBottom: msg.text ? 8 : 0 }}>
                  <Image
                    src={`data:image/jpeg;base64,${msg.image_b64}`}
                    style={{ maxWidth: 240, maxHeight: 240, borderRadius: 4 }}
                    preview
                  />
                </div>
              )}
              {/* File attachment */}
              {msg.has_file && (
                <div style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: 8,
                  padding: '6px 10px',
                  borderRadius: 6,
                  background: msg.direction === 'out' ? 'rgba(255,255,255,0.15)' : '#f5f5f5',
                  marginBottom: msg.text ? 8 : 0,
                }}>
                  <FileOutlined style={{ fontSize: 24, opacity: 0.7 }} />
                  <div>
                    <div style={{ fontSize: 13, fontWeight: 500 }}>{msg.file_name || 'file'}</div>
                    {msg.file_size ? (
                      <div style={{ fontSize: 11, opacity: 0.7 }}>{formatBytes(msg.file_size)}</div>
                    ) : null}
                  </div>
                </div>
              )}
              {/* Text */}
              {msg.text && <div style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>{msg.text}</div>}
              {/* Timestamp */}
              <div style={{
                fontSize: 11,
                opacity: 0.6,
                marginTop: 4,
                textAlign: msg.direction === 'out' ? 'right' : 'left',
              }}>
                {formatTime(msg.timestamp)}
              </div>
            </div>
          </div>
        ))}
        <div ref={bottomRef} />
      </div>

      {/* Input */}
      <div style={{ padding: '12px 20px', borderTop: '1px solid #f0f0f0', background: '#fff' }}>
        <Space.Compact style={{ width: '100%' }}>
          <Tooltip title="Send typing indicator">
            <Button
              icon={<EditOutlined />}
              onClick={() => sendTyping(botId, userId).catch(() => {})}
            />
          </Tooltip>
          <Upload
            showUploadList={false}
            beforeUpload={handleFileUpload}
          >
            <Tooltip title="Send image or file">
              <Button icon={<PaperClipOutlined />} />
            </Tooltip>
          </Upload>
          <Input
            placeholder="Type a message..."
            value={input}
            onChange={e => handleInputChange(e.target.value)}
            onPressEnter={handleSend}
            disabled={sending}
            style={{ flex: 1 }}
          />
          <Button
            type="primary"
            icon={<SendOutlined />}
            onClick={handleSend}
            loading={sending}
          >
            Send
          </Button>
        </Space.Compact>
      </div>
    </div>
  )
}
