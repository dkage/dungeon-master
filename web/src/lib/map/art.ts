import type { EffortTier } from '../api'
import bat from '../../assets/map/creatures/bat-idle-strip.png'
import beetle from '../../assets/map/creatures/beetle-idle-strip.png'
import dragon from '../../assets/map/creatures/dragon-idle-strip.png'
import ghoul from '../../assets/map/creatures/ghoul-idle-strip.png'
import goblin from '../../assets/map/creatures/goblin-idle-strip.png'
import kobold from '../../assets/map/creatures/kobold-idle-strip.png'
import lich from '../../assets/map/creatures/lich-idle-strip.png'
import manyEyed from '../../assets/map/creatures/many-eyed-idle-strip.png'
import orc from '../../assets/map/creatures/orc-idle-strip.png'
import rat from '../../assets/map/creatures/rat-idle-strip.png'
import skeleton from '../../assets/map/creatures/skeleton-idle-strip.png'
import slime from '../../assets/map/creatures/slime-idle-strip.png'
import spider from '../../assets/map/creatures/spider-idle-strip.png'
import troll from '../../assets/map/creatures/troll-idle-strip.png'
import warlock from '../../assets/map/creatures/warlock-idle-strip.png'
import burrow from '../../assets/map/dungeon-1-burrow.png'
import crypt from '../../assets/map/dungeon-2-crypt.png'
import tower from '../../assets/map/dungeon-3-tower.png'
import fortress from '../../assets/map/dungeon-4-fortress.png'
import citadel from '../../assets/map/dungeon-5-citadel.png'
import tavern from '../../assets/map/tavern.png'
import stoneFloor from '../../assets/map/stone-floor.jpg'
import worldFloor from '../../assets/map/world-floor.jpg'
import tavernFloor from '../../assets/map/tavern-floor.jpg'

export const TIERS: EffortTier[] = ['trivial', 'low', 'moderate', 'high', 'extreme']

/** The effort word shown on rooms and rows (docs/design/map.md, Room anatomy). */
export function effortWord(tier: EffortTier | undefined): string {
  return tier ? tier[0].toUpperCase() + tier.slice(1) : 'Unknown'
}

/**
 * Each creature's idle strip: four frames side by side, downscaled from the
 * sheets in design/quest-encounters/assets/ (docs/design/creature-roster.md).
 */
export const CREATURES: Record<string, { name: string; strip: string }> = {
  'dungeon-rat': { name: 'Dungeon rat', strip: rat },
  'cave-slime': { name: 'Cave slime', strip: slime },
  'cave-bat': { name: 'Cave bat', strip: bat },
  'kobold-scout': { name: 'Kobold scout', strip: kobold },
  'cave-beetle': { name: 'Cave beetle', strip: beetle },
  'cave-spider': { name: 'Cave spider', strip: spider },
  'goblin-scout': { name: 'Goblin scout', strip: goblin },
  'skeleton-guard': { name: 'Skeleton guard', strip: skeleton },
  'crypt-ghoul': { name: 'Crypt ghoul', strip: ghoul },
  'orc-raider': { name: 'Orc raider', strip: orc },
  'stone-troll': { name: 'Stone troll', strip: troll },
  'dungeon-warlock': { name: 'Dungeon warlock', strip: warlock },
  'many-eyed-stone-watcher': { name: 'Many-eyed stone watcher', strip: manyEyed },
  'crypt-lich': { name: 'Crypt lich', strip: lich },
  'ember-dragon': { name: 'Ember dragon', strip: dragon },
}

/** One frame of a strip is 3 wide by 4 high (543 × 724 in the source sheets). */
export const FRAME_ASPECT = 3 / 4

/** Dungeon art from smallest to grandest; index with dungeonTier() - 1. */
export const DUNGEON_ART = [burrow, crypt, tower, fortress, citadel]
export const DUNGEON_NAMES = ['Burrow', 'Crypt', 'Tower', 'Fortress', 'Citadel']
export const TAVERN_ART = tavern

export const FLOORS = { world: worldFloor, dungeon: stoneFloor, tavern: tavernFloor }
