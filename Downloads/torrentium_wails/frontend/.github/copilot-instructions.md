<!-- Use this file to provide workspace-specific custom instructions to Copilot. For more details, visit https://code.visualstudio.com/docs/copilot/copilot-customization#_use-a-githubcopilotinstructionsmd-file -->

# Torrentium Frontend Development Guidelines

This is a React frontend for a P2P file sharing network called Torrentium. Follow these guidelines when contributing to the project:

## Tech Stack
- React 19 with functional components and hooks
- Vite for build tooling
- Tailwind CSS for styling
- Lucide React for icons
- React Router DOM for routing
- Custom shadcn/ui inspired components

## Code Style
- Use functional components with hooks
- Prefer arrow functions for components
- Use camelCase for variables and functions
- Use PascalCase for component names
- Use descriptive variable names
- Add proper JSX comments when needed

## Component Structure
- Keep components focused and single-purpose
- Use custom hooks for complex logic
- Implement proper error boundaries
- Follow the existing folder structure in `src/components/`

## State Management
- Use React Context (`AppContext`) for global state
- Use `useReducer` for complex state updates
- Store user preferences in localStorage
- Implement optimistic updates for better UX

## Styling Guidelines
- Use Tailwind CSS classes
- Follow the established color palette:
  - Primary: #b4c5f9
  - Secondary: #0e6BA8
  - Background: #001b52
  - Surface: #000938
  - Text: #ffffff
  - Text Secondary: #b4c5f9
- Use the `cn()` utility function for conditional classes
- Implement responsive design with mobile-first approach

## UI Components
- Use existing UI components from `src/components/ui/`
- Follow the shadcn/ui pattern for new components
- Ensure accessibility with proper ARIA labels
- Implement proper focus management

## Data Handling
- Use mock data from `src/data/mockData.js` for development
- Implement realistic delays for async operations
- Handle loading and error states properly
- Use proper TypeScript-like prop validation with JSDoc

## Features to Maintain
- Drag-and-drop file uploads
- Real-time progress simulation
- Toast notifications for user feedback
- Responsive sidebar navigation
- Filter and search functionality
- User authentication simulation
- Peer-to-peer network simulation

## Performance
- Implement proper memoization where needed
- Use React.lazy for code splitting if components grow large
- Optimize re-renders with proper dependency arrays
- Minimize bundle size

## Accessibility
- Include proper ARIA labels
- Ensure keyboard navigation works
- Maintain proper color contrast
- Implement focus indicators
- Use semantic HTML elements

## Testing Considerations
- Write components that are easy to test
- Use data-testid attributes for testing
- Mock external dependencies properly
- Test user interactions and state changes
