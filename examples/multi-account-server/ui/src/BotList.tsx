import { useEffect, useState, useCallback } from 'react'
import { List, Button, Tag, Input, Modal, Space, Typography } from 'antd'
import { PlusOutlined, ReloadOutlined, QrcodeOutlined } from '@ant-design/icons'
import { QRCodeSVG } from 'qrcode.react'
import { listBots, activateBot, deactivateBot, getBotState, type BotInfo } from './api'

const { Text } = Typography

const stateColors: Record<string, string> = {
  running: 'green',
  session_expired: 'red',
  logging_in: 'blue',
  ready: 'cyan',
  stopped: 'default',
  new: 'default',
}

interface Props {
  selectedBot: string | null
  onSelect: (bot: BotInfo) => void
}

export default function BotList({ selectedBot, onSelect }: Props) {
  const [bots, setBots] = useState<BotInfo[]>([])
  const [newId, setNewId] = useState('')
  const [qrUrl, setQrUrl] = useState<string | null>(null)
  const [qrBotId, setQrBotId] = useState<string | null>(null)

  const refresh = useCallback(async () => {
    const data = await listBots()
    setBots(data)
  }, [])

  useEffect(() => {
    refresh()
    const t = setInterval(refresh, 3000)
    return () => clearInterval(t)
  }, [refresh])

  // Poll state while QR modal is open
  useEffect(() => {
    if (!qrBotId) return
    const t = setInterval(async () => {
      const { state } = await getBotState(qrBotId)
      if (state === 'running' || state === 'ready') {
        setQrUrl(null)
        setQrBotId(null)
        refresh()
      }
    }, 2000)
    return () => clearInterval(t)
  }, [qrBotId, refresh])

  const handleActivate = async (id: string) => {
    const resp = await activateBot(id)
    if (resp.qr_url) {
      setQrUrl(resp.qr_url)
      setQrBotId(id)
    }
    refresh()
  }

  const handleAdd = async () => {
    const id = newId.trim()
    if (!id) return
    setNewId('')
    await handleActivate(id)
  }

  const handleDeactivate = async (id: string) => {
    await deactivateBot(id)
    refresh()
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
      <div style={{ padding: 16, borderBottom: '1px solid #f0f0f0' }}>
        <Text strong style={{ fontSize: 16 }}>WeChat Bots</Text>
        <Button icon={<ReloadOutlined />} size="small" type="text" onClick={refresh} style={{ float: 'right' }} />
      </div>

      <div style={{ padding: '8px 16px' }}>
        <Space.Compact style={{ width: '100%' }}>
          <Input
            placeholder="Bot ID"
            value={newId}
            onChange={e => setNewId(e.target.value)}
            onPressEnter={handleAdd}
            size="small"
          />
          <Button icon={<PlusOutlined />} size="small" onClick={handleAdd} type="primary">
            Add
          </Button>
        </Space.Compact>
      </div>

      <div style={{ flex: 1, overflow: 'auto' }}>
        <List
          dataSource={bots}
          renderItem={bot => (
            <List.Item
              style={{
                padding: '8px 16px',
                cursor: 'pointer',
                background: selectedBot === bot.client_id ? '#e6f4ff' : undefined,
              }}
              onClick={() => onSelect(bot)}
              actions={[
                bot.state === 'running' ? (
                  <Button size="small" danger onClick={e => { e.stopPropagation(); handleDeactivate(bot.client_id) }}>
                    Stop
                  </Button>
                ) : (
                  <Button size="small" type="primary" icon={<QrcodeOutlined />}
                    onClick={e => { e.stopPropagation(); handleActivate(bot.client_id) }}>
                    Start
                  </Button>
                ),
              ]}
            >
              <List.Item.Meta
                title={<Text ellipsis style={{ maxWidth: 120 }}>{bot.client_id}</Text>}
                description={<Tag color={stateColors[bot.state] || 'default'}>{bot.state}</Tag>}
              />
            </List.Item>
          )}
        />
      </div>

      <Modal
        title="Scan QR Code"
        open={!!qrUrl}
        onCancel={() => { setQrUrl(null); setQrBotId(null) }}
        footer={null}
        width={360}
      >
        <div style={{ textAlign: 'center', padding: 20 }}>
          {qrUrl && <QRCodeSVG value={qrUrl} size={256} />}
          <div style={{ marginTop: 16, color: '#999', fontSize: 13 }}>
            Scan with WeChat to login
          </div>
        </div>
      </Modal>
    </div>
  )
}
