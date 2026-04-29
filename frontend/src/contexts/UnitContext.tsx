import React, { createContext, useContext, useState } from 'react';

export type DisplayUnit = 'FLC' | 'LOKI' | 'AUTO';

interface UnitContextType {
  unit: DisplayUnit;
  setUnit: (unit: DisplayUnit) => void;
}

const UnitContext = createContext<UnitContextType | undefined>(undefined);

export const UnitProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [unit, setUnitState] = useState<DisplayUnit>(() => {
    const saved = localStorage.getItem('flspd_unit');
    return (saved as DisplayUnit) || 'AUTO';
  });

  const setUnit = (newUnit: DisplayUnit) => {
    localStorage.setItem('flspd_unit', newUnit);
    setUnitState(newUnit);
  };

  return (
    <UnitContext.Provider value={{ unit, setUnit }}>
      {children}
    </UnitContext.Provider>
  );
};

export const useUnit = () => {
  const context = useContext(UnitContext);
  if (context === undefined) {
    throw new Error('useUnit must be used within a UnitProvider');
  }
  return context;
};
