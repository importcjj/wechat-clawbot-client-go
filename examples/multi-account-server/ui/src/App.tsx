import { useState } from 'react'
import { ConfigProvider, Layout, theme } from 'antd'
import BotList from './BotList'
import ChatPanel from './ChatPanel'
import type { BotInfo } from './api'

const { Sider, Content } = Layout

export default function App() {
  const [selectedBot, setSelectedBot] = useState<BotInfo | null>(null)

  return (
    <ConfigProvider theme={{ algorithm: theme.defaultAlgorithm }}>
      <Layout style={{ height: '100vh' }}>
        <Sider width={280} theme="light" style={{ borderRight: '1px solid #f0f0f0' }}>
          <BotList
            selectedBot={selectedBot?.client_id ?? null}
            onSelect={setSelectedBot}
          />
        </Sider>
        <Content style={{ background: '#fff' }}>
          {selectedBot && selectedBot.user_id ? (
            <ChatPanel botId={selectedBot.client_id} userId={selectedBot.user_id} />
          ) : selectedBot ? (
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%', color: '#999' }}>
              Bot not connected yet — click Start to scan QR code
            </div>
          ) : (
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%', color: '#999' }}>
              Select a bot from the left panel
            </div>
          )}
        </Content>
      </Layout>
    </ConfigProvider>
  )
}
