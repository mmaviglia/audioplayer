# AudioPlayer

`audioplayer` is a simple golang package providing basic audio functionality for various audio file types.

## Dependencies

FFmpeg is required for `audioplayer` to function.

Before using `audioplayer`, FFmpeg must either be accessible via the system `$PATH`, or the path to the FFmpeg binary must be set via the `SetFFmpegPath` function.

ALSA is also required, and can be installed on Ubuntu/Debian with the following command:

```sh
apt install libasound2-dev
```
