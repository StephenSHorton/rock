import type { LookPatch } from 'mokei/clay'
import {
  QUARRY_ACCENT_1,
  QUARRY_ACCENT_2,
  QUARRY_ACCENT_3,
  QUARRY_BASE,
  QUARRY_DETAIL_DARK,
  QUARRY_SANDSTONE,
} from 'mokei/theme/quarry'

/**
 * Thin look-store mapping onto the kit quarry theme.
 * Clay meshes take `material` roles. Do not re-declare hex here.
 */
export const quarryLook = {
  ground: QUARRY_SANDSTONE,
  road: QUARRY_SANDSTONE,
  grass: QUARRY_SANDSTONE,
  wall: QUARRY_BASE,
  roof: QUARRY_ACCENT_3,
  accent: QUARRY_ACCENT_1,
  yellow: QUARRY_ACCENT_2,
  cardboard: QUARRY_BASE,
  tree: QUARRY_SANDSTONE,
  tire: QUARRY_DETAIL_DARK,
  aoColor: QUARRY_ACCENT_3,
  skyColor: '#EEF1F6',
  groundBounce: QUARRY_SANDSTONE,
  sunColor: '#FFFAF4',
  sunAzimuth: -32,
  sunElevation: 48,
  sunIntensity: 0.3,
  skyIntensity: 0.78,
  aoIntensity: 2.4,
  aoRadius: 2.1,
} satisfies LookPatch
