const icons = {
  book: <path d="M12 5v16M12 5C8 2 4 3 2 4v15c4-2 7-1 10 2 3-3 6-4 10-2V4c-2-1-6-2-10 1Z" />,
  target: <path d="M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0ZM17 12a5 5 0 1 1-10 0 5 5 0 0 1 10 0ZM12 11v2" />,
  award: <path d="M17 8a5 5 0 1 1-10 0 5 5 0 0 1 10 0ZM8 12l-2 9 6-3 6 3-2-9" />,
  chart: <path d="M3 3v18h18M7 17v-5M12 17V8M17 17V5" />,
  shield: <path d="m12 2 8 4v6c0 5-8 10-8 10S4 17 4 12V6l8-4Zm-4 10 3 3 5-6" />,
  briefcase: <path d="M3 7h18v14H3V7Zm5 0V3h8v4M3 12c6 4 12 4 18 0M12 12v5" />,
  globe: <path d="M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0ZM12 3c-5 5-5 13 0 18 5-5 5-13 0-18ZM3 12h18M5 6h14M5 18h14" />,
  lightbulb: <path d="M8 16c0-3-3-3-3-7a7 7 0 0 1 14 0c0 4-3 4-3 7M8 17h8M9 20h6M11 23h2" />,
  video: <path d="M3 4h18v16H3V4Zm7 4 6 4-6 4V8Z" />,
  rocket: <path d="M9 15c-1-6 5-12 12-12 0 7-6 13-12 12ZM9 9H5l-3 6h7M15 15v4l-6 3v-7M6 18l-3 3M18 8a2 2 0 1 1-4 0 2 2 0 0 1 4 0Z" />,
  search: <><circle cx="11" cy="11" r="7" /><path d="m20 20-4-4" /></>,
  bot: <><rect x="4" y="7" width="16" height="13" rx="4" /><path d="M12 3v4M8 12h.01M16 12h.01M8 16h8" /></>,
  workflow: <><rect x="3" y="3" width="7" height="5" rx="1" /><rect x="14" y="16" width="7" height="5" rx="1" /><path d="M6.5 8v5h11v3M17.5 13V8H14" /></>,
  sparkles: <><path d="m12 3 1.5 4.5L18 9l-4.5 1.5L12 15l-1.5-4.5L6 9l4.5-1.5L12 3Z" /><path d="m19 15 .8 2.2L22 18l-2.2.8L19 21l-.8-2.2L16 18l2.2-.8L19 15Z" /></>,
  heart: <path d="M20.8 4.6a5.5 5.5 0 0 0-7.8 0L12 5.7l-1.1-1.1a5.5 5.5 0 0 0-7.8 7.8l1.1 1.1L12 21l7.8-7.5 1.1-1.1a5.5 5.5 0 0 0-.1-7.8Z" />,
  bag: <><path d="M6 8h12l1 13H5L6 8Z" /><path d="M9 8V5a3 3 0 0 1 6 0v3" /></>,
  folder: <path d="M3 6h7l2 2h9v11H3V6Z" />,
  clock: <><circle cx="12" cy="12" r="9" /><path d="M12 7v5l3 2" /></>,
  message: <path d="M4 5h16v12H8l-4 4V5Z" />,
  calculator: <><rect x="5" y="2" width="14" height="20" rx="2" /><path d="M8 6h8M8 11h.01M12 11h.01M16 11h.01M8 15h.01M12 15h.01M16 15h.01M8 19h.01M12 19h4" /></>,
  users: <><circle cx="9" cy="8" r="3" /><circle cx="17" cy="9" r="2" /><path d="M3 20c0-4 2-7 6-7s6 3 6 7M15 14c4 0 6 2 6 6" /></>,
  list: <><path d="M9 6h12M9 12h12M9 18h12M4 6h.01M4 12h.01M4 18h.01" /></>,
  check: <><rect x="3" y="3" width="18" height="18" rx="3" /><path d="m7 12 3 3 7-7" /></>,
  replace: <><path d="M20 7h-9a5 5 0 0 0-5 5v1" /><path d="m17 4 3 3-3 3M4 17h9a5 5 0 0 0 5-5v-1" /><path d="m7 20-3-3 3-3" /></>,
  trash: <><path d="M4 7h16M9 7V4h6v3M7 7l1 14h8l1-14M10 11v6M14 11v6" /></>,
}

export default function Icon({ name, ...props }) {
  return <svg viewBox="0 0 24 24" aria-hidden="true" {...props}>{icons[name] || icons.workflow}</svg>
}
