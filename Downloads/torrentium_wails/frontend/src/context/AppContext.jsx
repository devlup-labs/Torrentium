import React, { createContext, useContext, useReducer, useEffect } from 'react'
import { mockUser, mockTransfers, mockPeers } from '../data/mockData'

const AppContext = createContext()

const initialState = {
  user: mockUser,
  transfers: mockTransfers,
  peers: mockPeers,
  notifications: [],
  theme: 'dark'
}

function appReducer(state, action) {
  switch (action.type) {
    case 'UPDATE_USER':
      return { ...state, user: { ...state.user, ...action.payload } }
    
    case 'ADD_TRANSFER':
      return { 
        ...state, 
        transfers: [action.payload, ...state.transfers] 
      }
    
    case 'UPDATE_TRANSFER':
      return {
        ...state,
        transfers: state.transfers.map(transfer =>
          transfer.id === action.payload.id
            ? { ...transfer, ...action.payload }
            : transfer
        )
      }
    
    case 'DELETE_TRANSFER':
      return {
        ...state,
        transfers: state.transfers.filter(transfer => transfer.id !== action.payload)
      }
    
    case 'ADD_PEER':
      return {
        ...state,
        peers: [action.payload, ...state.peers]
      }
    
    case 'UPDATE_PEER':
      return {
        ...state,
        peers: state.peers.map(peer =>
          peer.id === action.payload.id
            ? { ...peer, ...action.payload }
            : peer
        )
      }
    
    case 'BLOCK_PEER':
      return {
        ...state,
        peers: state.peers.map(peer =>
          peer.id === action.payload
            ? { ...peer, isBlocked: true }
            : peer
        )
      }
    
    case 'UNBLOCK_PEER':
      return {
        ...state,
        peers: state.peers.map(peer =>
          peer.id === action.payload
            ? { ...peer, isBlocked: false }
            : peer
        )
      }
    
    case 'ADD_NOTIFICATION':
      return {
        ...state,
        notifications: [...state.notifications, action.payload]
      }
    
    case 'REMOVE_NOTIFICATION':
      return {
        ...state,
        notifications: state.notifications.filter(notif => notif.id !== action.payload)
      }
    
    case 'SET_THEME':
      return {
        ...state,
        theme: action.payload
      }
    
    default:
      return state
  }
}

export function AppProvider({ children }) {
  const [state, dispatch] = useReducer(appReducer, initialState)

  // Load state from localStorage on mount
  useEffect(() => {
    const savedState = localStorage.getItem('torrentium-state')
    if (savedState) {
      try {
        const parsed = JSON.parse(savedState)
        dispatch({ type: 'UPDATE_USER', payload: parsed.user || mockUser })
      } catch (error) {
        console.error('Failed to load saved state:', error)
      }
    }
  }, [])

  // Save state to localStorage on changes
  useEffect(() => {
    localStorage.setItem('torrentium-state', JSON.stringify({
      user: state.user,
      theme: state.theme
    }))
  }, [state.user, state.theme])

  const addNotification = (message, type = 'info') => {
    const notification = {
      id: Date.now().toString(),
      message,
      type,
      timestamp: new Date()
    }
    dispatch({ type: 'ADD_NOTIFICATION', payload: notification })
    
    // Auto remove after 5 seconds
    setTimeout(() => {
      dispatch({ type: 'REMOVE_NOTIFICATION', payload: notification.id })
    }, 5000)
  }

  const value = {
    ...state,
    dispatch,
    addNotification
  }

  return (
    <AppContext.Provider value={value}>
      {children}
    </AppContext.Provider>
  )
}

export function useApp() {
  const context = useContext(AppContext)
  if (!context) {
    throw new Error('useApp must be used within an AppProvider')
  }
  return context
}
