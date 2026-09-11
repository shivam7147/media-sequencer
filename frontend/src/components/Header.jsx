export default function Header({ cycleSeconds, activeSync }) {
  return (
    <header className="wall-header">
      <div className="header-main">
        <div className="header-titles">
          <h1>Media Sequencer</h1>
          <p className="header-status">
            Cycle {cycleSeconds}s
            {activeSync ? ' · sync overlay active' : ''}
          </p>
        </div>
        <div className="header-author-section">
          <span className="header-author">
            Made by <strong>Shivam Rao</strong>
          </span>
          <div className="header-links">
            <a
              href="https://github.com/shivam7147/media-sequencer"
              target="_blank"
              rel="noopener noreferrer"
              className="header-link"
              title="GitHub Repository"
            >
              <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
                <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z" />
              </svg>
              <span>Repository</span>
            </a>
            <a
              href="https://www.linkedin.com/in/shivam-rao-940327290/"
              target="_blank"
              rel="noopener noreferrer"
              className="header-link"
              title="LinkedIn Profile"
            >
              <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
                <path d="M19 3a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h14m-.5 15.5v-5.3a3.26 3.26 0 0 0-3.26-3.26c-.85 0-1.84.52-2.28 1.3v-1.11h-2.79v8.37h2.79v-4.93c0-.77.62-1.4 1.39-1.4a1.4 1.4 0 0 1 1.4 1.4v4.93h2.75M6.46 10.9v8.37H9.25V10.9H6.46M7.86 6.74a1.63 1.63 0 1 0 0 3.26 1.63 1.63 0 0 0 0-3.26z" />
              </svg>
              <span>LinkedIn</span>
            </a>
            <a
              href="https://drive.google.com/file/d/1lDJF-YgZpb-gVxXkOp4Jk4Nj12UVwFpB/view?usp=sharing"
              target="_blank"
              rel="noopener noreferrer"
              className="header-link"
              title="Resume"
            >
              <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
                <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8l-6-6zm2 16H8v-2h8v2zm0-4H8v-2h8v2zm-3-5V3.5L18.5 9H13z" />
              </svg>
              <span>Resume</span>
            </a>
          </div>
        </div>
      </div>
    </header>
  )
}
