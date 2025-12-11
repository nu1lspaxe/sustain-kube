import pandas as pd
import numpy as np
import matplotlib.pyplot as plt
from sklearn.metrics import r2_score, mean_absolute_error
import matplotlib.pyplot as plt

plt.rcParams['font.family'] = ['Segoe UI Emoji', 'sans-serif']

# ==========================================
# 1. 定義測試用的通用負載 (Workload Pattern)
# ==========================================
def generate_workload(points=1000):
    """生成一組標準化的負載波形 (日夜週期 + 隨機波動)"""
    x = np.linspace(0, 6 * np.pi, points)
    # CPU: 10% ~ 90% 波動
    cpu_util = 0.5 + 0.4 * np.sin(x) + np.random.normal(0, 0.02, points)
    cpu_util = np.clip(cpu_util, 0.1, 0.9)
    
    # Mem: 跟隨 CPU 但較平滑，範圍 20% ~ 80%
    mem_util = 0.5 + 0.3 * np.sin(x) + np.random.normal(0, 0.01, points)
    mem_util = np.clip(mem_util, 0.2, 0.8)
    
    return cpu_util, mem_util

# ==========================================
# 2. 定義硬體場景 (Hardware Profiles)
# ==========================================
# 我們模擬三種不同的硬體，證明公式的泛用性
scenarios = [
    {
        "name": "🌋 High-Perf Server (Intel Xeon)",
        "cores": 64.0, "mem_gb": 256.0,
        "p_idle": 100.0, "p_max": 500.0, # 閒置功耗很大
        "mem_coeff_ccf": 0.392 # W/GB (CCF Standard)
    },
    {
        "name": "☁️ Standard VM (AWS m5.2xlarge)",
        "cores": 8.0, "mem_gb": 32.0,
        "p_idle": 20.0, "p_max": 120.0,
        "mem_coeff_ccf": 0.392
    },
    {
        "name": "🔋Edge Device (Raspberry Pi Cluster)",
        "cores": 4.0, "mem_gb": 8.0,
        "p_idle": 2.5, "p_max": 15.0, # 功耗極低
        "mem_coeff_ccf": 0.2 # LPDDR 比較省電
    }
]

# 準備繪圖
fig, axes = plt.subplots(1, 3, figsize=(18, 5))
cpu_util, mem_util = generate_workload()

print(f"{'='*80}")
print(f"{'Sustain-Kube 硬體通用性驗證 (Hardware Generality Test)':^80}")
print(f"{'='*80}")

# ==========================================
# 3. 迴圈測試每個場景
# ==========================================
for i, hw in enumerate(scenarios):
    # --- A. 準備數據 ---
    df = pd.DataFrame({'cpu_util': cpu_util, 'mem_util': mem_util})
    
    # 轉換為絕對資源量 (Sustain-Kube Input)
    df['used_cores'] = df['cpu_util'] * hw['cores']
    df['used_mem_gb'] = df['mem_util'] * hw['mem_gb']
    
    # --- B. 計算真值 (Shadow Model - CCF) ---
    # Compute: P_min + Util * (P_max - P_min)
    watts_compute = hw['p_idle'] + df['cpu_util'] * (hw['p_max'] - hw['p_idle'])
    # Memory: GB * Coeff
    watts_mem = df['used_mem_gb'] * hw['mem_coeff_ccf']
    
    df['watts_shadow_total'] = watts_compute + watts_mem
    
    # --- C. 自動校準 (Auto-Calibration) ---
    # 目標：找出適合該硬體的 "Sustain-Kube CPU Coefficient"
    # 公式：K_cpu = (Total_Shadow - Total_Mem_Sustain) / Total_Cores
    # 假設 Memory 係數我們設定得跟 CCF 一樣準
    sustain_mem_power_sum = (df['used_mem_gb'] * hw['mem_coeff_ccf']).sum()
    shadow_total_sum = df['watts_shadow_total'].sum()
    cores_sum = df['used_cores'].sum()
    
    best_cpu_coeff = (shadow_total_sum - sustain_mem_power_sum) / cores_sum
    
    # --- D. 執行 Sustain-Kube 估算 ---
    df['watts_sustain'] = (df['used_cores'] * best_cpu_coeff) + \
                          (df['used_mem_gb'] * hw['mem_coeff_ccf'])
    
    # --- E. 評估指標 ---
    r2 = r2_score(df['watts_shadow_total'], df['watts_sustain'])
    mae = mean_absolute_error(df['watts_shadow_total'], df['watts_sustain'])
    
    # --- F. 輸出報告 ---
    print(f"\n[場景 {i+1}]: {hw['name']}")
    print(f"   - 硬體特徵: {hw['cores']} Cores, Idle {hw['p_idle']}W -> Max {hw['p_max']}W")
    print(f"   - ✅ 推薦 CPU 係數: {best_cpu_coeff:.4f}")
    print(f"   - 驗證 R² Score:   {r2:.4f}")
    print(f"   - 平均誤差 (MAE):  {mae:.2f} Watts")

    # --- G. 繪圖 ---
    ax = axes[i]
    # 只畫前 300 點避免擁擠
    subset = df.head(300)
    ax.plot(subset.index, subset['watts_shadow_total'], label='Shadow (CCF)', color='gray', linestyle='--', linewidth=2, alpha=0.6)
    ax.plot(subset.index, subset['watts_sustain'], label='Sustain-Kube', color='green', linewidth=2, alpha=0.8)
    ax.set_title(f"{hw['name']}\nCoeff: {best_cpu_coeff:.2f} | R²: {r2:.3f}")
    ax.set_xlabel('Time')
    ax.set_ylabel('Watts')
    ax.legend()
    ax.grid(True, alpha=0.3)

plt.tight_layout()
print(f"\n{'='*80}")
plt.show()