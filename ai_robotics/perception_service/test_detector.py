from __future__ import annotations

import unittest

import numpy as np

import detector


class DetectorTest(unittest.TestCase):
    def setUp(self) -> None:
        self.scene = detector.generate_sample_scene()

    def labels(self, objects: list[dict]) -> set[str]:
        return {obj["label"] for obj in objects}

    def test_sample_scene_contains_red_and_blue(self) -> None:
        objects = detector.detect(self.scene)
        labels = self.labels(objects)
        self.assertIn("red_block", labels)
        self.assertIn("blue_marker", labels)

    def test_bboxes_are_within_image(self) -> None:
        height, width = self.scene.shape[:2]
        for obj in detector.detect(self.scene):
            x1, y1, x2, y2 = obj["bbox"]
            self.assertLess(x1, x2)
            self.assertLess(y1, y2)
            self.assertGreaterEqual(x1, 0)
            self.assertGreaterEqual(y1, 0)
            self.assertLessEqual(x2, width)
            self.assertLessEqual(y2, height)
            self.assertGreaterEqual(obj["confidence"], 0.5)
            self.assertLessEqual(obj["confidence"], 0.99)

    def test_red_block_position_hint_is_left(self) -> None:
        objects = [o for o in detector.detect(self.scene) if o["label"] == "red_block"]
        self.assertEqual(len(objects), 1)
        self.assertEqual(objects[0]["position_hint"], "front_left")

    def test_empty_image_detects_nothing(self) -> None:
        blank = np.zeros((240, 320, 3), dtype=np.uint8)
        self.assertEqual(detector.detect(blank), [])

    def test_base64_roundtrip(self) -> None:
        encoded = detector.encode_image_base64(self.scene)
        self.assertTrue(encoded)
        decoded = detector.decode_base64_image(encoded)
        self.assertIsNotNone(decoded)
        assert decoded is not None
        self.assertEqual(decoded.shape[:2], self.scene.shape[:2])
        labels = self.labels(detector.detect(decoded))
        self.assertIn("red_block", labels)

    def test_invalid_base64_returns_none(self) -> None:
        self.assertIsNone(detector.decode_base64_image("not-an-image"))


if __name__ == "__main__":
    unittest.main()
