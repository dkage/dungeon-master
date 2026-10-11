export const WORDS = {
  campaign: { themed: 'Campaign', plain: 'Project', meaning: 'A folder you add. Its issues come from its Git remote.' },
  quest: { themed: 'Quest', plain: 'Issue', meaning: 'A GitHub issue you can send an agent to work on.' },
  hero: { themed: 'Hero', plain: 'Agent setup', meaning: 'A Claude model and its launch options.' },
  spell: { themed: 'Spell', plain: 'Skill', meaning: 'Reusable instructions an agent can follow.' },
  expedition: { themed: 'Expedition', plain: 'Session', meaning: 'A live terminal running an agent or shell.' },
  briefing: { themed: 'Briefing', plain: 'Task notes', meaning: 'Context prepared from an issue before an agent starts.' },
  dungeon: { themed: 'Dungeon', plain: 'Map', meaning: 'A visual map of issues and their dependencies.' },
} as const

export type WordKey = keyof typeof WORDS

const plural = {
  campaign: { themed: 'Campaigns', plain: 'Projects' },
  quest: { themed: 'Quests', plain: 'Issues' },
  hero: { themed: 'Heroes', plain: 'Agent setups' },
  spell: { themed: 'Spells', plain: 'Skills' },
  expedition: { themed: 'Expeditions', plain: 'Sessions' },
  briefing: { themed: 'Briefings', plain: 'Task notes' },
  dungeon: { themed: 'Dungeons', plain: 'Maps' },
} satisfies Record<WordKey, { themed: string; plain: string }>

export function wordLabel(key: WordKey, mode: 'hints' | 'themed' | 'plain', many = false): string {
  const labels = many ? plural[key] : WORDS[key]
  return mode === 'plain' ? labels.plain : labels.themed
}
