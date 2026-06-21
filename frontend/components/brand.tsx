export function Logo({ className }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 32 32" fill="none" aria-hidden="true">
      <rect x="1" y="1" width="30" height="30" rx="9" fill="#131519" stroke="#262a32" />
      {/* a four-point "north star" — direction-setting */}
      <path
        d="M16 7l2.2 6.4L24.6 16l-6.4 2.2L16 24.6l-2.2-6.4L7.4 16l6.4-2.2L16 7z"
        fill="#e3a857"
      />
      <circle cx="16" cy="16" r="2.1" fill="#1c1404" />
    </svg>
  );
}

export function Wordmark({ className }: { className?: string }) {
  return (
    <span className={className}>
      <span className="flex items-center gap-2.5">
        <Logo className="size-7" />
        <span className="text-[15px] font-semibold tracking-tight text-fg">
          AI Chief of Staff
        </span>
      </span>
    </span>
  );
}
