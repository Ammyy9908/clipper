# Brag Plan: clipPlay

## What is this app?
An iOS app (backend codename "clipper") that turns any copied video link — YouTube, Instagram, TikTok and ~1800 more — into a saved video, a vertical 9:16 split, or an MP3 in a built-in music player, in one tap.

## The angle
The app's own promise is its cleanest joke: **"Copy. Paste. Done."** — and it's true, because the app *already knows* you copied a link (clipboard detection on launch, no paste banner). So the video is a tap-by-tap demo film: one gesture in, three outcomes out (video, vertical clips, audio). No mascot, no parody — the product doing its thing, fast, on the warm-paper-and-coral look the app already has.

## Hook (first 2-3 seconds)
Three giant words land one at a time on warm off-white: **Copy.** **Paste.** **Done.** — the last one in coral. It's the app's onboarding headline, so it's verbatim, and it sets the whole video's promise in 3 seconds.

## Key moments (the middle)
- **The link is already there.** Phone slides in on the Download tab: "A link is ready to paste" → "Link copied" card → a finger taps **Use copied link** → "Fetching video details" spinner → the resolved card appears (thumbnail, title, uploader, quality chips, **Split into clips** toggle on at 30s) → tap **Save & Split**.
- **One long video in, vertical clips out.** Tab bar hops to **Split**. A group card fills with three 9:16 parts popping in one by one: Part 1, Part 2, Part 3 — "Auto 9:16 Reframe".
- **Or just keep the sound.** Tab hops to **Music** — "Your Audio", Quick Picks grid filling in card by card, mini player sliding up with a playing progress bar.

## Outro / punchline
Hard cut on the loudest beat to the coral app tile (play-circle glyph, the app's splash) slamming in with **clipPlay** and the app's own tagline: **Download. Organize. Play.** Small row beneath: YouTube · Instagram · TikTok.

## User flow worth showing
1. **Entry** — open the app with a link on the clipboard: "A link is ready to paste".
2. **Key action** — tap "Use copied link" → resolved card → toggle "Split into clips" → "Save & Split".
3. **Result** — Split tab shows the video as 3 vertical parts; Music tab shows audio saved and playing.

## Tone
- Preset: `default`
- Creative direction: "quiet-confident consumer app launch — the demo does the talking" (clean product film, warm and postable, no parody)
- Interpretation: Comfortable pacing with cuts on the beat, one idea per scene, short headline copy in the app's own voice; motion is springy/rounded like the app, humor comes only from how instant it all is.

## Format: landscape — 1920x1080
## Duration: 20 seconds

## Visual identity (from the project — `ios/Clipper/Theme.swift`)
- Background: `#F9F6F2` (warm off-white; dark mode `#121110`)
- Surface / cards: `#FFFFFF`
- Text: `#1C1A17` (ink), secondary `#857D73`, tertiary `#B8B0A6`
- Accent: `#FF4A38` (coral-red, same in both modes)
- Hairline: `rgba(0,0,0,0.08)`
- Display font: SF Pro Rounded, bold/heavy (rounded system design) → web stand-in: Nunito (800–900)
- Body font: SF Pro (system medium) → web stand-in: Inter / system sans
- Strongest visual element: the Download-tab card (coral primary button on white card, hairline border, 20px radius) and the coral app tile from `SplashScreen.swift` (28px-radius tile, play.circle.fill glyph, soft coral shadow)

## Share copy (draft)
Copy. Paste. Done. clipPlay turns any video link into a saved video, vertical clips, or an MP3 — YouTube, Instagram, TikTok and more.

## Audio direction
- Role: warm bed with motion-matched accents
- Music: `happy-beats-business-moves-vol-1-by-ende-dot-app.mp3` (120 BPM, most energetic — fits `default`)
- Music treatment: starts at 0:00 at ~0.35, no fade-in needed (the track's beat lands at 3.02s = the hook→demo cut); fade under the last second (19.0→20.0s)
- Music cue guidance: bundled preset read (`assets/music/cues/…vol-1….music-cues.md`). Tempo 120.19 BPM; beat grid 3.02, 3.52, 4.02 … at ~0.5s. Strong cues to lock: **16.02s** (logo slam — the outro cut), **17.02s** (tagline). Scene cuts land on grid beats 3.02 / 8.02 / 12.02. Sequential windows: split parts on 9.02 / 9.52 / 10.02; Quick Pick cards on 13.02 / 13.52 / 14.02 / 14.52.
- Audio-reactive treatment: subtle — music RMS/bass gently swells the coral glow behind the phone and the app tile's shadow. No waveforms, no bars, no pulsing text.
- SFX posture: moderate, motion-matched, low HF-risk picks (soft impacts, clicks, drops)
- Audio-coupled moments: three hook words land with soft drops; two simulated taps get clicks; parts/cards pop with soft slides; outro tile gets a bell hit on the strong cue
- Restraint rule: no SFX over the tagline read; no stacking more than two cues in the same half-second; music never above 0.4

## Storyboard

### Scene 1 — Hook — 3.02s (0.00–3.02)
Warm off-white. Giant rounded type stacks left-aligned center: **Copy.** (in at 0.20), **Paste.** (in at 1.00), **Done.** in coral (in at 1.80). Each slams in fast (~0.3s) and stays. Small coral dot + "clipPlay" wordmark top-left fades in at 0.2. All three words settled together from ~2.1s to the cut.
Sequential/interaction: yes — three words arrive one by one, ~0.8s apart (one word each, so reading floor is met).
Audio intent: light, confident; music already rolling.
Audio-coupled idea: a soft drop on each word landing.
Music: bed in.
Transition mood: hard cut on the 3.02 beat → Scene 2

### Scene 2 — Paste — 5.00s (3.02–8.02)
Headline left: **Copied a link? clipPlay already knows.** (in at ~3.3, holds through the scene; 7 words → ≥2.1s settled). Phone slides in from the right at 3.02 with springy ease and settles by ~3.6. Screen = Download tab, verbatim copy: "clipPlay" / "A link is ready to paste"; card: "Link copied" / "Tap below to fetch this video" / coral button **Use copied link**. At 4.53 a cursor-tap lands on the button (press-down, ripple). 4.53–5.53: "Fetching video details" + spinner. At 5.53 the resolved card appears: thumbnail with play glyph, title "Sunset drive · 4K", uploader "Roadtrip Diaries", quality chips 1080p / 720p / MP3, toggle **Split into clips** on ("Cuts the video into shorter parts") with a 30s pill, button **Save & Split**. At 7.02 a second tap presses the button.
Sequential/interaction: yes — simulated cursor taps at 4.53 and 7.02; card content stages in at 5.53.
Audio intent: tactile, satisfying.
Audio-coupled idea: click at each tap; soft drop when the resolved card lands.
Music: bed continues.
Transition mood: slide/wipe (screen content swaps, phone stays) → Scene 3

### Scene 3 — Split — 4.00s (8.02–12.02)
Tab bar hops to **Split** (icon tints coral). Headline left swaps to **One long video in. Vertical clips out.** with a small pill "Auto 9:16 Reframe". Phone screen = Split tab: title "Split", group row "Sunset drive · 4K — 3 parts" and three 9:16 thumbnails popping in one by one: Part 1 at 9.02, Part 2 at 9.52, Part 3 at 10.02, then all held ~2s.
Sequential/interaction: yes — three parts pop on consecutive beats (labels are tiny "Part N" tags; full set holds 2s+).
Audio intent: quick rhythmic ticks that ride the beat.
Audio-coupled idea: card-slide accents on parts 1–3.
Music: bed continues.
Transition mood: slide → Scene 4

### Scene 4 — Music — 4.00s (12.02–16.02)
Tab bar hops to **Music**. Headline left: **Or keep just the sound.** with a sub line "Playlists. Background playback. Lock-screen controls." Phone screen = Music tab: "Your Audio" / "4 audio tracks", "Quick Picks" 2×2 grid of cards filling in at 13.02, 13.52, 14.02, 14.52 (artwork tiles in warm tints with coral accent), then the mini player slides up at 15.02 with a playing progress bar.
Sequential/interaction: yes — four cards on consecutive beats, mini player at 15.02.
Audio intent: warm and building toward the outro.
Audio-coupled idea: soft drops on the cards (first and last accented), click on mini player.
Music: bed swelling toward the strong cue at 16.02.
Transition mood: hard cut on the 16.02 strong cue → Scene 5

### Scene 5 — Outro — 3.98s (16.02–20.00)
Full-bleed warm off-white. The coral app tile (play.circle.fill glyph, soft coral shadow) scales in with a spring on 16.02 (**beat-locked strong cue**). "clipPlay" in heavy rounded type at ~16.35. Tagline **Download. Organize. Play.** lands on 17.02 (strong cue). Small platform row "YouTube · Instagram · TikTok" at 17.52. Hold to ~19.5, gentle fade of everything into the background and music by 20.0.
Sequential/interaction: yes — tile → name → tagline → platforms, ≥0.5s apart, each holding to the end.
Audio intent: payoff; brief bell then let music carry.
Audio-coupled idea: one bell hit on the tile slam, one soft impact on the tagline.
Music: last bars; fade out 19.0→20.0.
Transition mood: fade to background at end.

**Music mood for this video:** upbeat
**Audio summary:** Upbeat business-happy bed from frame 0, cuts on the beat, tactile taps in the demo, a single bell on the logo slam.
