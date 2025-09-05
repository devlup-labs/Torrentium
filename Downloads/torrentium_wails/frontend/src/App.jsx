import React from 'react'
import { BrowserRouter as Router, Routes, Route } from 'react-router-dom'
import { AppProvider } from './context/AppContext'
import { Layout } from './components/Layout'
import { Home } from './pages/Home'
import { Upload } from './pages/Upload'
import { Download } from './pages/Download'
import { History } from './pages/History'
import { PrivateNetwork } from './pages/PrivateNetwork'
import { Profile } from './pages/Profile'
import { Leaderboard } from './pages/Leaderboard'
import Settings from './pages/Settings'
import Premium from './pages/Premium'

function App() {
  return (
    <AppProvider>
      <Router 
        future={{
          v7_startTransition: true,
          v7_relativeSplatPath: true
        }}
      >
        <Routes>
          <Route path="/" element={<Layout />}>
            <Route index element={<Home />} />
            <Route path="upload" element={<Upload />} />
            <Route path="download" element={<Download />} />
            <Route path="history" element={<History />} />
            <Route path="network" element={<PrivateNetwork />} />
            <Route path="profile" element={<Profile />} />
            <Route path="leaderboard" element={<Leaderboard />} />
            <Route path="settings" element={<Settings />} />
            <Route path="premium" element={<Premium />} />
          </Route>
        </Routes>
      </Router>
    </AppProvider>
  )
}

export default App
