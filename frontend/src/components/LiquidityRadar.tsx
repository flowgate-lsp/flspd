import React from 'react';
import {
  Radar,
  RadarChart,
  PolarGrid,
  PolarAngleAxis,
  PolarRadiusAxis,
  ResponsiveContainer,
} from 'recharts';

interface RadarData {
  subject: string;
  A: number;
  fullMark: number;
}

const defaultData: RadarData[] = [
  { subject: 'Liquidity', A: 120, fullMark: 150 },
  { subject: 'Uptime', A: 98, fullMark: 150 },
  { subject: 'Speed', A: 86, fullMark: 150 },
  { subject: 'Success Rate', A: 99, fullMark: 150 },
  { subject: 'Relay Fees', A: 85, fullMark: 150 },
  { subject: 'Compliance', A: 145, fullMark: 150 },
];

interface LiquidityRadarProps {
  data?: RadarData[] | null;
}

export const LiquidityRadar: React.FC<LiquidityRadarProps> = ({ data }) => {
  const chartData = data || defaultData;

  return (
    <div className="w-full h-full min-h-[300px] flex items-center justify-center">
      <ResponsiveContainer width="100%" height="100%">
        <RadarChart cx="50%" cy="50%" outerRadius="80%" data={chartData}>
          <PolarGrid stroke="rgba(255, 255, 255, 0.1)" />
          <PolarAngleAxis
            dataKey="subject"
            tick={{ fill: 'rgba(255, 255, 255, 0.6)', fontSize: 10, fontWeight: 'bold' }}
          />
          <PolarRadiusAxis
            angle={30}
            domain={[0, 150]}
            tick={false}
            axisLine={false}
          />
          <Radar
            name="Node Performance"
            dataKey="A"
            stroke="#00baff"
            fill="#00baff"
            fillOpacity={0.3}
            animationDuration={1500}
          />
        </RadarChart>
      </ResponsiveContainer>
    </div>
  );
};
