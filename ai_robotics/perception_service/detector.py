"""OpenCV HSV color detector for workbench objects.

This replaces the hardcoded mock with real image processing. The detector
boundary stays the same, so a YOLO/SAM-backed implementation can replace
this module later without changing the HTTP contract.
"""
from __future__ import annotations

from typing import Any

import cv2
import numpy as np

# Minimum contour area in pixels to count as a detection.
MIN_AREA = 400

# HSV ranges per label. Red wraps around hue 0 so it needs two ranges.
COLOR_SPECS: list[dict[str, Any]] = [
    {
        "label": "red_block",
        "display_name": "赤いブロック",
        "ranges": [
            (np.array([0, 90, 80]), np.array([10, 255, 255])),
            (np.array([170, 90, 80]), np.array([180, 255, 255])),
        ],
    },
    {
        "label": "blue_marker",
        "display_name": "青い目印",
        "ranges": [
            (np.array([100, 90, 80]), np.array([130, 255, 255])),
        ],
    },
    {
        "label": "green_block",
        "display_name": "緑のブロック",
        "ranges": [
            (np.array([40, 90, 80]), np.array([80, 255, 255])),
        ],
    },
    {
        "label": "yellow_block",
        "display_name": "黄色いブロック",
        "ranges": [
            (np.array([20, 90, 80]), np.array([35, 255, 255])),
        ],
    },
]


def detect(image_bgr: np.ndarray) -> list[dict[str, Any]]:
    """Detect colored objects and return them sorted by confidence."""
    height, width = image_bgr.shape[:2]
    hsv = cv2.cvtColor(image_bgr, cv2.COLOR_BGR2HSV)

    objects: list[dict[str, Any]] = []
    for spec in COLOR_SPECS:
        mask = np.zeros((height, width), dtype=np.uint8)
        for low, high in spec["ranges"]:
            mask |= cv2.inRange(hsv, low, high)
        mask = cv2.morphologyEx(mask, cv2.MORPH_OPEN, np.ones((5, 5), np.uint8))

        contours, _ = cv2.findContours(mask, cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_SIMPLE)
        for contour in contours:
            area = cv2.contourArea(contour)
            if area < MIN_AREA:
                continue
            x, y, w, h = cv2.boundingRect(contour)
            # Fill ratio of the bounding box approximates shape solidity;
            # solid blocks score higher than scattered noise.
            fill_ratio = area / float(w * h)
            confidence = round(min(0.99, 0.5 + 0.5 * fill_ratio), 2)
            objects.append({
                "label": spec["label"],
                "display_name": spec["display_name"],
                "confidence": confidence,
                "bbox": [int(x), int(y), int(x + w), int(y + h)],
                "position_hint": position_hint(x + w / 2, y + h / 2, width, height),
            })

    objects.sort(key=lambda obj: obj["confidence"], reverse=True)
    return objects


def position_hint(cx: float, cy: float, width: int, height: int) -> str:
    if cy > height * 0.75:
        return "near_front"
    if cx < width / 3:
        return "front_left"
    if cx > width * 2 / 3:
        return "front_right"
    return "front_center"


def generate_sample_scene(width: int = 480, height: int = 320) -> np.ndarray:
    """Render a synthetic workbench so detection works without an upload."""
    image = np.full((height, width, 3), (96, 116, 134), dtype=np.uint8)

    # Workbench surface with slight vertical gradient.
    for row in range(height):
        shade = int(20 * row / height)
        image[row, :] = (96 + shade, 116 + shade, 134 + shade)

    # Red block, front left.
    cv2.rectangle(image, (94, 88), (200, 178), (40, 40, 205), thickness=-1)
    cv2.rectangle(image, (94, 88), (200, 178), (30, 30, 160), thickness=3)

    # Blue marker, front right.
    cv2.circle(image, (330, 160), 46, (190, 110, 30), thickness=-1)
    cv2.circle(image, (330, 160), 46, (150, 80, 20), thickness=3)

    # Dark table edge strip near the bottom.
    cv2.rectangle(image, (0, 270), (width, 300), (52, 56, 60), thickness=-1)

    return image


def decode_base64_image(data: str) -> np.ndarray | None:
    import base64

    try:
        raw = base64.b64decode(data, validate=False)
    except Exception:
        return None
    array = np.frombuffer(raw, dtype=np.uint8)
    image = cv2.imdecode(array, cv2.IMREAD_COLOR)
    return image


def encode_image_base64(image_bgr: np.ndarray) -> str:
    import base64

    ok, buffer = cv2.imencode(".jpg", image_bgr, [cv2.IMWRITE_JPEG_QUALITY, 85])
    if not ok:
        return ""
    return base64.b64encode(buffer.tobytes()).decode("ascii")
