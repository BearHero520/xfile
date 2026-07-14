import { useEffect, useMemo, useRef, useState } from "react";
import videojs from "video.js";
import type Player from "video.js/dist/types/player";
import "video.js/dist/video-js.css";

const videoTypes: Record<string, string> = {
  mp4: "video/mp4",
  m4v: "video/x-m4v",
  mov: "video/quicktime",
  webm: "video/webm",
  ogv: "video/ogg",
  ogg: "video/ogg",
  m3u8: "application/x-mpegURL",
  mpd: "application/dash+xml",
};

function fileExtension(name: string) {
  return name.split(".").pop()?.toLowerCase() || "";
}

export function isVideoFileName(name: string) {
  return fileExtension(name) in videoTypes;
}

export default function VideoPreview({
  src,
  name,
  className = "",
  immersive = false,
}: {
  src: string;
  name: string;
  className?: string;
  immersive?: boolean;
}) {
  const videoHostRef = useRef<HTMLDivElement>(null);
  const playerRef = useRef<Player | null>(null);
  const [playbackError, setPlaybackError] = useState("");
  const type = useMemo(
    () => videoTypes[fileExtension(name)] || "video/mp4",
    [name],
  );

  useEffect(() => {
    const host = videoHostRef.current;
    if (!host) return;

    setPlaybackError("");
    const videoElement = document.createElement("video-js");
    videoElement.classList.add("vjs-big-play-centered");
    videoElement.setAttribute("aria-label", `${name} 视频播放器`);
    videoElement.setAttribute("playsinline", "");
    host.appendChild(videoElement);

    const player = videojs(videoElement, {
      controls: true,
      fluid: !immersive,
      fill: immersive,
      responsive: true,
      preload: "metadata",
      playsinline: true,
      sources: [{ src, type }],
    });
    playerRef.current = player;

    const handlePlaybackReady = () => setPlaybackError("");
    const handlePlaybackError = () => {
      const playerError = player.error();
      setPlaybackError(
        playbackErrorMessage(playerError?.code, playerError?.message),
      );
    };

    player.on("loadedmetadata", handlePlaybackReady);
    player.on("playing", handlePlaybackReady);
    player.on("error", handlePlaybackError);

    return () => {
      player.off("loadedmetadata", handlePlaybackReady);
      player.off("playing", handlePlaybackReady);
      player.off("error", handlePlaybackError);
      if (!player.isDisposed()) {
        player.dispose();
      }
      if (playerRef.current === player) playerRef.current = null;
      host.replaceChildren();
    };
  }, [immersive, name, src, type]);

  return (
    <div
      className={`xfile-video-preview ${immersive ? "is-immersive" : ""} ${className}`}
    >
      <div ref={videoHostRef} data-vjs-player />
      {(!immersive || playbackError) && (
        <p className={playbackError ? "is-error" : undefined} role="status">
          {playbackError || "支持在线播放；若当前浏览器无法解码，可"}
          <a href={src} download={name}>
            下载原文件
          </a>
          {playbackError ? "后在本地播放器中打开。" : "后播放。"}
        </p>
      )}
    </div>
  );
}

function playbackErrorMessage(code?: number, fallback?: string) {
  switch (code) {
    case 1:
      return "视频播放已中止。";
    case 2:
      return "视频加载失败，请检查网络后重试。";
    case 3:
      return "浏览器无法解码此视频，可能是编码配置不兼容。";
    case 4:
      return "当前浏览器不支持此视频格式或编码。";
    default:
      return fallback ? `视频播放失败：${fallback}` : "视频播放失败。";
  }
}
