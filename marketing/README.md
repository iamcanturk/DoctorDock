# Marketing assets

- **promo-square.mp4** — a 1080×1080, ~16s promo for social media (X, LinkedIn,
  Instagram feed). No audio, no copyrighted material. Built from the real app
  screenshots and the mascot with `ffmpeg`.

Regenerate: the scenes are drawn with Pillow and assembled with ffmpeg; the
script lived in the session that produced this. To remake it, render fresh app
screenshots (`DoctorDock --render`), frame them, and crossfade the scene PNGs.
