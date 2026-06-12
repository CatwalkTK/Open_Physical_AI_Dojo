import React from "react";
import * as THREE from "three";
import { OrbitControls } from "three/examples/jsm/controls/OrbitControls.js";
import type { SimulatorObject, SimulatorState } from "../lib/api/types";

// Simulator coordinates: x = forward (m), y = left (m), yaw in degrees.
// Three.js mapping: three.x = sim.x, three.z = -sim.y, rotation.y = yaw(rad).
const DEG = Math.PI / 180;

type Pose = { x: number; y: number; yaw: number };

type SceneRefs = {
  renderer: THREE.WebGLRenderer;
  camera: THREE.PerspectiveCamera;
  scene: THREE.Scene;
  controls: OrbitControls;
  robot: THREE.Group;
  legs: THREE.Group[];
  body: THREE.Mesh;
  pathLine: THREE.Line;
  objectsGroup: THREE.Group;
  frame: number;
  lastTime: number;
  gaitPhase: number;
  current: Pose;
  target: Pose;
  mode: string;
  pathKey: string;
  objectsKey: string;
};

const DEFAULT_TARGETS: SimulatorObject[] = [
  { id: "red_block", label: "赤いブロック", x: 0.42, y: 0.12, radius: 0.08 },
  { id: "blue_marker", label: "青い目印", x: 0.34, y: -0.24, radius: 0.07 },
];

const DEFAULT_OBSTACLES: SimulatorObject[] = [
  { id: "table_edge", label: "机の端", x: 0.58, y: 0, radius: 0.06 },
];

export function Simulator3D({ state }: { state?: SimulatorState }) {
  const mountRef = React.useRef<HTMLDivElement | null>(null);
  const refs = React.useRef<SceneRefs | null>(null);

  React.useEffect(() => {
    const mount = mountRef.current;
    if (!mount) return;

    const scene = new THREE.Scene();
    scene.background = new THREE.Color(0xe8eeec);
    scene.fog = new THREE.Fog(0xe8eeec, 2.2, 4.5);

    const camera = new THREE.PerspectiveCamera(45, 4 / 3, 0.05, 20);
    camera.position.set(-0.55, 0.65, 0.85);

    const renderer = new THREE.WebGLRenderer({ antialias: true });
    renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
    mount.appendChild(renderer.domElement);

    const controls = new OrbitControls(camera, renderer.domElement);
    controls.target.set(0.25, 0.05, 0);
    controls.enableDamping = true;
    controls.maxPolarAngle = Math.PI / 2.05;
    controls.minDistance = 0.3;
    controls.maxDistance = 3;

    scene.add(new THREE.AmbientLight(0xffffff, 0.75));
    const sun = new THREE.DirectionalLight(0xffffff, 1.4);
    sun.position.set(-1, 2, 1.2);
    scene.add(sun);

    // Workbench floor and grid.
    const floor = new THREE.Mesh(
      new THREE.PlaneGeometry(2.4, 2.4),
      new THREE.MeshLambertMaterial({ color: 0xb8c8c4 }),
    );
    floor.rotation.x = -Math.PI / 2;
    floor.position.set(0.3, -0.001, 0);
    scene.add(floor);

    const grid = new THREE.GridHelper(2.4, 24, 0x8aa39d, 0xa3b8b3);
    grid.position.set(0.3, 0, 0);
    scene.add(grid);

    // Origin marker (start position).
    const origin = new THREE.Mesh(
      new THREE.RingGeometry(0.035, 0.05, 32),
      new THREE.MeshBasicMaterial({ color: 0x5c7d75, side: THREE.DoubleSide }),
    );
    origin.rotation.x = -Math.PI / 2;
    origin.position.y = 0.001;
    scene.add(origin);

    // Quadruped robot: body, head, 4 swinging legs.
    const robot = new THREE.Group();
    const bodyMat = new THREE.MeshStandardMaterial({ color: 0x2d3a43, roughness: 0.55 });
    const accentMat = new THREE.MeshStandardMaterial({ color: 0xffd447, roughness: 0.5 });

    const body = new THREE.Mesh(new THREE.BoxGeometry(0.2, 0.055, 0.11), bodyMat);
    body.position.y = 0.1;
    robot.add(body);

    const head = new THREE.Mesh(new THREE.BoxGeometry(0.05, 0.04, 0.07), accentMat);
    head.position.set(0.115, 0.115, 0);
    robot.add(head);

    const eye = new THREE.Mesh(
      new THREE.CylinderGeometry(0.012, 0.012, 0.01, 16),
      new THREE.MeshStandardMaterial({ color: 0x172026 }),
    );
    eye.rotation.z = Math.PI / 2;
    eye.position.set(0.143, 0.115, 0);
    robot.add(eye);

    const legGeom = new THREE.BoxGeometry(0.018, 0.095, 0.018);
    legGeom.translate(0, -0.0475, 0); // pivot at hip
    const legs: THREE.Group[] = [];
    const hipY = 0.095;
    const legPositions: [number, number][] = [
      [0.075, 0.045],
      [0.075, -0.045],
      [-0.075, 0.045],
      [-0.075, -0.045],
    ];
    for (const [lx, lz] of legPositions) {
      const hip = new THREE.Group();
      hip.position.set(lx, hipY, lz);
      const leg = new THREE.Mesh(legGeom, bodyMat);
      hip.add(leg);
      const foot = new THREE.Mesh(new THREE.SphereGeometry(0.012, 12, 12), accentMat);
      foot.position.y = -0.095;
      hip.add(foot);
      robot.add(hip);
      legs.push(hip);
    }
    scene.add(robot);

    // Trajectory line.
    const pathGeometry = new THREE.BufferGeometry();
    const pathLine = new THREE.Line(
      pathGeometry,
      new THREE.LineBasicMaterial({ color: 0xe0712c }),
    );
    pathLine.position.y = 0.004;
    scene.add(pathLine);

    const objectsGroup = new THREE.Group();
    scene.add(objectsGroup);

    const sceneRefs: SceneRefs = {
      renderer,
      camera,
      scene,
      controls,
      robot,
      legs,
      body,
      pathLine,
      objectsGroup,
      frame: 0,
      lastTime: performance.now(),
      gaitPhase: 0,
      current: { x: 0, y: 0, yaw: 0 },
      target: { x: 0, y: 0, yaw: 0 },
      mode: "idle",
      pathKey: "",
      objectsKey: "",
    };
    refs.current = sceneRefs;

    const resize = () => {
      const width = mount.clientWidth;
      const height = Math.max(1, Math.round(width * 0.75));
      renderer.setSize(width, height);
      camera.aspect = width / height;
      camera.updateProjectionMatrix();
    };
    resize();
    const observer = new ResizeObserver(resize);
    observer.observe(mount);

    const animate = (time: number) => {
      const r = refs.current;
      if (!r) return;
      const dt = Math.min(0.1, (time - r.lastTime) / 1000);
      r.lastTime = time;

      // Ease displayed pose toward the latest simulator pose.
      const ease = Math.min(1, dt * 6);
      r.current.x += (r.target.x - r.current.x) * ease;
      r.current.y += (r.target.y - r.current.y) * ease;
      let yawDiff = r.target.yaw - r.current.yaw;
      while (yawDiff > 180) yawDiff -= 360;
      while (yawDiff < -180) yawDiff += 360;
      r.current.yaw += yawDiff * ease;

      r.robot.position.set(r.current.x, 0, -r.current.y);
      r.robot.rotation.y = r.current.yaw * DEG;

      // Walking gait: swing legs while moving, settle when stopped.
      const distanceToTarget = Math.hypot(r.target.x - r.current.x, r.target.y - r.current.y);
      const walking = r.mode === "moving" || distanceToTarget > 0.005 || Math.abs(yawDiff) > 2;
      if (walking) {
        r.gaitPhase += dt * 11;
        r.legs.forEach((leg, index) => {
          const phase = r.gaitPhase + (index % 2 === 0 ? 0 : Math.PI);
          leg.rotation.x = Math.sin(phase) * 0.45;
        });
        r.body.position.y = 0.1 + Math.abs(Math.sin(r.gaitPhase * 2)) * 0.004;
      } else {
        r.legs.forEach((leg) => {
          leg.rotation.x *= Math.max(0, 1 - dt * 8);
        });
        const sitting = r.mode === "sitting";
        const targetBodyY = sitting ? 0.07 : 0.1;
        r.body.position.y += (targetBodyY - r.body.position.y) * Math.min(1, dt * 6);
      }

      r.controls.update();
      r.renderer.render(r.scene, r.camera);
      r.frame = requestAnimationFrame(animate);
    };
    sceneRefs.frame = requestAnimationFrame(animate);

    return () => {
      observer.disconnect();
      cancelAnimationFrame(sceneRefs.frame);
      controls.dispose();
      renderer.dispose();
      mount.removeChild(renderer.domElement);
      refs.current = null;
    };
  }, []);

  // Sync simulator state into the scene.
  React.useEffect(() => {
    const r = refs.current;
    if (!r) return;

    r.target = {
      x: state?.x ?? 0,
      y: state?.y ?? 0,
      yaw: state?.yaw_deg ?? 0,
    };
    r.mode = state?.mode ?? "idle";

    const path = state?.path ?? [];
    const pathKey = `${path.length}:${path[path.length - 1]?.time ?? ""}`;
    if (pathKey !== r.pathKey) {
      r.pathKey = pathKey;
      const points = path.map((pose) => new THREE.Vector3(pose.x, 0, -pose.y));
      r.pathLine.geometry.dispose();
      r.pathLine.geometry = new THREE.BufferGeometry().setFromPoints(points);
    }

    const targets = state?.targets ?? DEFAULT_TARGETS;
    const obstacles = state?.obstacles ?? DEFAULT_OBSTACLES;
    const objectsKey = JSON.stringify([targets, obstacles]);
    if (objectsKey !== r.objectsKey) {
      r.objectsKey = objectsKey;
      rebuildObjects(r.objectsGroup, targets, obstacles);
    }
  }, [state]);

  return <div ref={mountRef} className="sim3d-mount" aria-label="3D simulator view" />;
}

function rebuildObjects(group: THREE.Group, targets: SimulatorObject[], obstacles: SimulatorObject[]) {
  for (const child of [...group.children]) {
    group.remove(child);
    disposeObject(child);
  }

  for (const target of targets) {
    const color = colorForObject(target.id);
    let mesh: THREE.Mesh;
    if (target.id.includes("marker")) {
      mesh = new THREE.Mesh(
        new THREE.CylinderGeometry(target.radius * 0.8, target.radius * 0.8, 0.02, 24),
        new THREE.MeshStandardMaterial({ color, roughness: 0.6 }),
      );
      mesh.position.set(target.x, 0.01, -target.y);
    } else {
      const size = target.radius * 1.4;
      mesh = new THREE.Mesh(
        new THREE.BoxGeometry(size, size, size),
        new THREE.MeshStandardMaterial({ color, roughness: 0.6 }),
      );
      mesh.position.set(target.x, size / 2, -target.y);
    }
    group.add(mesh);
    group.add(makeRing(target.x, target.y, target.radius, color));
  }

  for (const obstacle of obstacles) {
    const wall = new THREE.Mesh(
      new THREE.BoxGeometry(0.02, 0.07, 0.7),
      new THREE.MeshStandardMaterial({ color: 0x4a4f54, roughness: 0.8 }),
    );
    wall.position.set(obstacle.x + obstacle.radius, 0.035, -obstacle.y);
    group.add(wall);
    group.add(makeRing(obstacle.x, obstacle.y, obstacle.radius, 0x4a4f54));
  }
}

function makeRing(x: number, y: number, radius: number, color: number) {
  const ring = new THREE.Mesh(
    new THREE.RingGeometry(radius, radius + 0.012, 32),
    new THREE.MeshBasicMaterial({ color, side: THREE.DoubleSide, transparent: true, opacity: 0.45 }),
  );
  ring.rotation.x = -Math.PI / 2;
  ring.position.set(x, 0.002, -y);
  return ring;
}

function colorForObject(id: string): number {
  if (id.includes("red")) return 0xd23939;
  if (id.includes("blue")) return 0x2e6fbe;
  if (id.includes("green")) return 0x2f9e44;
  if (id.includes("yellow")) return 0xe6b400;
  return 0x888888;
}

function disposeObject(object: THREE.Object3D) {
  object.traverse((node) => {
    const mesh = node as THREE.Mesh;
    if (mesh.geometry) mesh.geometry.dispose();
    const material = mesh.material as THREE.Material | THREE.Material[] | undefined;
    if (Array.isArray(material)) material.forEach((m) => m.dispose());
    else if (material) material.dispose();
  });
}
