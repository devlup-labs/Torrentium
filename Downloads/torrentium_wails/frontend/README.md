# Torrentium - P2P File Sharing Network Frontend

A modern React frontend for a decentralized P2P file-sharing network built with Vite, Tailwind CSS, and shadcn/ui components.

## Features

### Core Functionality
- **Dashboard**: Overview of recent activity, active transfers, and quick stats
- **File Upload**: Drag-and-drop interface for sharing files with the network
- **Download Manager**: Torrent file upload and download management with real-time progress
- **Transfer History**: Complete activity log with filtering and search capabilities
- **Private Network**: Peer management with trust scores and invite system
- **User Profile**: Personal stats, badges, and account settings
- **Leaderboard**: Community rankings and achievements

### Technical Features
- Responsive design with mobile-first approach
- Dark mode interface with custom color palette
- Real-time progress tracking (simulated)
- Drag-and-drop file handling
- Toast notifications for user feedback
- Local storage for user preferences
- Mock data with realistic P2P scenarios

## Tech Stack

- **Framework**: React 19 with Vite
- **Styling**: Tailwind CSS with custom design system
- **UI Components**: Custom shadcn/ui inspired components
- **Icons**: Lucide React
- **Routing**: React Router DOM
- **State Management**: React Context + useReducer

## Color Palette

```css
Primary: #b4c5f9 (Light blue accent)
Secondary: #0e6BA8 (Medium blue)
Background: #001b52 (Dark blue)
Surface: #000938 (Darker blue)
Text: #ffffff (White)
Text Secondary: #b4c5f9 (Light blue)
Success: #10b981
Warning: #f59e0b
Error: #ef4444
```

## Getting Started

### Prerequisites
- Node.js 18+
- npm or yarn

### Installation

1. Install dependencies:
```bash
npm install
```

2. Start the development server:
```bash
npm run dev
```

3. Open [http://localhost:5173](http://localhost:5173) to view it in the browser.

### Available Scripts

- `npm run dev` - Start development server
- `npm run build` - Build for production
- `npm run preview` - Preview production build
- `npm run lint` - Run ESLint

## Project Structure

```
src/
├── components/
│   ├── ui/          # Reusable UI components
│   ├── Layout.jsx   # Main layout wrapper
│   ├── Sidebar.jsx  # Navigation sidebar
│   └── Toast.jsx    # Notification system
├── pages/
│   ├── Home.jsx           # Dashboard
│   ├── Upload.jsx         # File upload interface
│   ├── Download.jsx       # Download manager
│   ├── History.jsx        # Transfer history
│   ├── PrivateNetwork.jsx # Peer management
│   ├── Profile.jsx        # User profile
│   └── Leaderboard.jsx    # Community rankings
├── context/
│   └── AppContext.jsx     # Global state management
├── data/
│   └── mockData.js        # Mock data for development
├── lib/
│   └── utils.js           # Utility functions
└── App.jsx                # Main app component
```

## Key Features

### Sidebar Navigation
- Auto-collapsing sidebar that expands on hover
- Smooth animations and transitions
- Tooltips for collapsed state
- Active route highlighting
- Mobile-friendly hamburger menu

### File Upload
- Drag-and-drop interface with visual feedback
- File validation and preview
- Batch upload support with progress tracking
- SHA-256 hash generation simulation

### Download Manager
- Torrent file upload via drag-and-drop
- Real-time progress bars with speed metrics
- Peer connection indicators
- Health status for torrents

### Transfer History
- Filterable and sortable table
- Search functionality
- Bulk actions (pause/resume/delete)
- Status badges with color coding

### Private Network
- Peer management with trust scores
- Invite code generation and sharing
- Block/unblock functionality
- Online/offline status tracking

### Profile Management
- Editable user information
- Achievement badge system
- Statistics dashboard
- Account settings

### Leaderboard
- Community rankings by various metrics
- Search and filtering
- Achievement categories
- User comparison features

## Mock Data

The application includes comprehensive mock data for:
- User profiles with trust scores and badges
- File transfers with various statuses
- Peer networks with online/offline states
- Leaderboard rankings
- Achievement badges

## State Management

- React Context for global state
- useReducer for complex state updates
- Local storage persistence
- Optimistic updates for better UX

## License

This project is licensed under the MIT License.
