# Hyperframes Composition Brief: clipPlay

## Objective
Create a short launch-style brag video for clipPlay (iOS app: copy a video link → saved video, vertical clips, or MP3 in a built-in player).

## Output
- Composition directory: `brag-output/composition/`
- Rendered video: `brag-output/brag.mp4`
- Format: landscape — 1920x1080
- Duration: 20 seconds

## Source Material
- Project root: `/Users/sumitkumar/Downloads/clipper`
- Primary files read: `README.md`, `ios/SETUP.md`, `ios/Clipper/Theme.swift`, `DownloadTabView.swift`, `OnboardingView.swift`, `SplashScreen.swift`, `ContentView.swift`, `MusicPlayerHomeView.swift`, `SplitTabView.swift`, `VideoTabView.swift`
- Product name: clipPlay
- Tagline / strongest claim: "Copy. Paste. Done." / "Download. Organize. Play."
- Key UI or visual moment to recreate: the Download tab card (link copied → fetching → resolved card with Split toggle → Save & Split), the Split tab with 3 vertical parts, the Music tab with Quick Picks and the mini player, the coral splash tile
- Copy that must appear verbatim:
  - Copy. / Paste. / Done.
  - A link is ready to paste · Link copied · Tap below to fetch this video · Use copied link
  - Fetching video details · Split into clips · Cuts the video into shorter parts · Save & Split
  - Auto 9:16 Reframe
  - Your Audio · Quick Picks
  - Download. Organize. Play.

## Creative Direction
- Tone preset: default
- Creative direction: quiet-confident consumer app launch — the demo does the talking
- Interpretation: comfortable pacing, cuts on the beat, one idea per scene; springy rounded motion like the app; no parody
- Angle: The app's own promise ("Copy. Paste. Done.") is the joke and the proof. Show one gesture in and three outcomes out: video, vertical clips, audio.
- Hook: three giant words land one by one — Copy. Paste. Done. (coral)
- Outro / punchline: coral app tile slams in on the strong cue — clipPlay — Download. Organize. Play.
- Avoid:
  - Generic SaaS language
  - Abstract filler visuals
  - Real third-party brands/logos or real video titles (use invented "Sunset drive · 4K" / "Roadtrip Diaries"; platform names appear only as plain text)

## Visual Identity
- Background: `#F9F6F2`
- Text: `#1C1A17` (secondary `#857D73`, tertiary `#B8B0A6`)
- Accent: `#FF4A38`
- Surface: `#FFFFFF`, hairline `rgba(0,0,0,0.08)`
- Display font: Nunito 800/900 (stand-in for SF Pro Rounded)
- Body font: Inter (stand-in for SF Pro)
- Visual references from the project: coral primary button on white 20px-radius card; coral 28px-radius splash tile with play.circle.fill and soft coral shadow; bottom tab bar (Download / Videos / Split / Music) tinted coral when active

## Storyboard
Use the storyboard in `brag-output/brag-plan.md` as the creative contract.

Scene summary:
1. Hook — 3.02s — Copy. Paste. Done.
2. Paste — 5.00s — phone on Download tab; taps; resolve; Save & Split
3. Split — 4.00s — Split tab; three 9:16 parts on consecutive beats
4. Music — 4.00s — Music tab; Quick Picks; mini player
5. Outro — 3.98s — coral tile, clipPlay, Download. Organize. Play.

## Audio
- Audio role: warm bed with motion-matched accents
- Audio arc: bed from 0:00; tactile taps in the demo; a bell on the logo slam; fade out over the last second
- Music: `happy-beats-business-moves-vol-1-by-ende-dot-app.mp3`
- Music treatment: 0.35 volume, fade 19.0→20.0
- Music cue guidance: bundled preset `assets/music/cues/happy-beats-business-moves-vol-1-by-ende-dot-app.music-cues.json` (120.19 BPM; beats 3.02, 3.52, …). Strong locks: 16.02 (outro slam), 17.02 (tagline). Scene cuts on 3.02 / 8.02 / 12.02.
- Audio-reactive treatment: subtle — per-frame RMS/bass (pre-extracted with ffmpeg, embedded in the composition) gently swells the coral glow behind the phone and the app-tile shadow. No waveform/equalizer visuals.
- Audio-coupled moments:
  - Scene 1 — three hook words — soft drops
  - Scene 2 — two simulated taps (4.53, 7.02) — clicks; resolved card at 5.53 — soft drop
  - Scene 3 — parts at 9.02 / 9.52 / 10.02 — soft card slides
  - Scene 4 — Quick Picks at 13.02–14.52 (accent first + last), mini player at 15.02 — click
  - Scene 5 — tile slam at 16.02 — bell; tagline at 17.02 — soft impact
- SFX selection guidance: low HF-risk sounds; sound at the start of the motion
- SFX analysis guidance: `.claude/skills/brag/assets/sfx/sfx-analysis.md`
- Exact SFX choice: final filenames/timestamps as implemented in `composition/index.html`
- Audio files: copied into `brag-output/composition/assets/`

## Hyperframes Instructions
Hyperframes domain skills (`hyperframes-core`, etc.) were not installed in this environment; the composition was built from the CLI's own docs (`hyperframes docs …`) and the scaffold's AGENTS.md rules (paused GSAP timeline on `window.__timelines`, `data-start`/`data-duration`, `class="clip"`, deterministic only). `hyperframes check` is the gate.
