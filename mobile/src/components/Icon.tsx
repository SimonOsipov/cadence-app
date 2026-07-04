import React from 'react';
import Svg, { Path } from 'react-native-svg';
import { C } from '../theme';
import { ICON_PATHS, IconName } from './icon-paths';

export type { IconName };

// Heroicons outline, 24/24 viewBox, stroke 1.5 — matches the prototype <Icon>.
export function Icon({
  name,
  size = 24,
  strokeWidth = 1.5,
  color = C.ink800,
}: {
  name: IconName;
  size?: number;
  strokeWidth?: number;
  color?: string;
}) {
  const paths = ICON_PATHS[name];
  if (!paths) return null;
  return (
    <Svg width={size} height={size} viewBox="0 0 24 24" fill="none">
      {paths.map((d, i) => (
        <Path
          key={i}
          d={d}
          stroke={color}
          strokeWidth={strokeWidth}
          strokeLinecap="round"
          strokeLinejoin="round"
        />
      ))}
    </Svg>
  );
}
