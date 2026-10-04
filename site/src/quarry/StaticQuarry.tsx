import { quarryLook, roles } from './palette'

export function StaticQuarry() {
  return (
    <svg className="h-full w-full" viewBox="0 0 960 540" preserveAspectRatio="xMidYMid slice" role="img" aria-label="Static clay quarry: a drill rig, two ore carts on rails, tool stations, a permission gate, and crates on a dock.">
      <rect width="960" height="540" fill={quarryLook.skyColor} />
      <path d="M0 360h960v180H0z" fill={roles.ground} />
      <path d="M40 300h520l80 80H0z" fill={roles.rock} />
      <path d="M220 210h420l70 90H160z" fill={roles.rock} />
      <path d="M80 170h220l40 80H40z" fill={roles.ground} />
      <ellipse cx="210" cy="410" rx="42" ry="18" fill={roles.rock} />
      <ellipse cx="470" cy="430" rx="34" ry="14" fill={roles.rock} />
      <ellipse cx="760" cy="420" rx="38" ry="16" fill={roles.rock} />
      <path d="M90 390c70-18 160-8 250 4 90 12 190 4 280-8 70-10 160 4 230 16" fill="none" stroke={roles.base} strokeWidth="10" strokeLinecap="round" />
      <path d="M90 402c70-18 160-8 250 4 90 12 190 4 280-8 70-10 160 4 230 16" fill="none" stroke={roles.detail} strokeWidth="4" strokeLinecap="round" />
      <g transform="translate(70 250)">
        <rect x="0" y="90" width="90" height="18" rx="6" fill={roles.ground} />
        <rect x="8" y="40" width="48" height="52" rx="8" fill={roles.base} />
        <rect x="6" y="32" width="52" height="12" rx="5" fill={roles.accent1} />
        <rect x="58" y="-20" width="12" height="130" rx="6" fill={roles.accent1} />
        <rect x="50" y="-32" width="28" height="14" rx="5" fill={roles.accent1} />
      </g>
      <g transform="translate(250 300)">
        <rect width="54" height="40" rx="8" fill={roles.base} />
        <rect y="-8" width="54" height="10" rx="4" fill={roles.accent1} />
      </g>
      <g transform="translate(390 308)">
        <rect width="54" height="40" rx="8" fill={roles.base} />
        <rect y="-8" width="54" height="10" rx="4" fill={roles.accent1} />
      </g>
      <g transform="translate(520 292)">
        <rect x="0" y="8" width="10" height="46" rx="4" fill={roles.base} />
        <rect x="48" y="8" width="10" height="46" rx="4" fill={roles.base} />
        <rect x="2" y="6" width="70" height="10" rx="4" fill={roles.accent2} transform="rotate(-8 2 6)" />
      </g>
      <g transform="translate(640 318)">
        <rect width="54" height="40" rx="8" fill={roles.base} />
        <rect y="-8" width="54" height="10" rx="4" fill={roles.accent1} />
      </g>
      <g transform="translate(790 300)">
        <rect x="-10" y="40" width="130" height="22" rx="8" fill={roles.base} />
        <rect x="8" y="8" width="36" height="26" rx="6" fill={roles.base} />
        <rect x="8" y="18" width="36" height="6" rx="2" fill={roles.accent1} />
        <rect x="48" y="8" width="36" height="26" rx="6" fill={roles.base} />
        <rect x="48" y="18" width="36" height="6" rx="2" fill={roles.accent1} />
        <rect x="28" y="-18" width="36" height="26" rx="6" fill={roles.base} />
        <rect x="28" y="-8" width="36" height="6" rx="2" fill={roles.accent1} />
      </g>
      <g transform="translate(300 368)">
        <rect width="44" height="22" rx="6" fill={roles.base} />
        <rect x="4" y="4" width="36" height="8" rx="3" fill={roles.accent1} />
        <circle cx="10" cy="24" r="6" fill={roles.detail} />
        <circle cx="34" cy="24" r="6" fill={roles.detail} />
      </g>
      <g transform="translate(700 352)">
        <rect width="44" height="22" rx="6" fill={roles.base} />
        <rect x="4" y="4" width="36" height="8" rx="3" fill={roles.accent1} />
        <circle cx="10" cy="24" r="6" fill={roles.detail} />
        <circle cx="34" cy="24" r="6" fill={roles.detail} />
      </g>
    </svg>
  )
}
