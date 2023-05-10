import matplotlib.pyplot as plt
import pandas as pd

import pandas as pd
import matplotlib.pyplot as plt
import matplotlib.dates as mdates
import matplotlib as mpl

from matplotlib.font_manager import FontManager
import subprocess
from matplotlib.font_manager import FontProperties
import numpy as np

fm = FontManager()
mat_fonts = set(f.name for f in fm.ttflist)
#print(mat_fonts)
output = subprocess.check_output(
    'fc-list :lang=zh -f "%{family}\n"', shell=True) # 获取字体列表
output = output.decode('utf-8')
#print(output)

zh_fonts = set(f.split(',', 1)[0] for f in output.split('\n'))
available = mat_fonts & zh_fonts
print('*' * 10, '可用的字体', '*' * 10)
for f in available:
    print(f)

font_path = "/usr/local/share/fonts/SourceHanSerifSC-Regular.otf"
# 加载中文字体
font = FontProperties(fname=font_path, size=14)
mpl.rcParams['axes.unicode_minus'] = False  # 正确显示负号，防止变成方框
# set font by font_path
plt.rcParams['font.sans-serif'] = ['SimHei']  # 选择中文字体
plt.rcParams['font.family'] = ['Source Han Serif SC']  # 选择中文字体
# 设置字号
plt.rcParams['font.size'] = 14

def draw():
    # 读取数据
    data = pd.read_csv("cap_per_balance_0.csv")
    data1 = pd.read_csv("cap_per_balance_1.csv")

    # 创建图形和左侧y轴
    fig, ax1 = plt.subplots()

    # 重排 x 轴，倒序
    data = data.sort_values(by='std', ascending=True)
    data1 = data1.sort_values(by='std', ascending=True)
    x_data = data.iloc[:, 4] / 1024 ** 3

    # 绘制左侧y轴折线图
    ax1.plot(x_data, data.iloc[:, 1], linestyle='--', color="red", label="Ceph")
    ax1.plot(x_data, data1.iloc[:, 1], color="blue", label="Ecos")
    ax1.set_xlabel("节点容量标准差")
    ax1.set_ylabel("节点写入量标准差")
    diff = data.iloc[:, 1] - data1.iloc[:, 1]
    max_diff = np.max(np.abs(diff))
    max_diff_idx = np.argmax(np.abs(diff))

    # 添加图例
    ax1.legend(loc="upper left")

    # 打印最大差值 和 此时的容量标准差
    print(f"max diff: {max_diff:.8f}, std: {x_data[max_diff_idx]:.2f}")





    plt.show()

    fig, ax2 = plt.subplots()

    # 将字节转换为GB，并绘制右侧y轴折线图
    right_y_data = data.iloc[:, 3] / 1024 ** 3
    ax2.plot(x_data, right_y_data, linestyle='--', color="red",  label="Ceph")
    right_y_data = data1.iloc[:, 3] / 1024 ** 3
    ax2.plot(x_data, right_y_data, color="blue",  label="Ecos")
    ax2.set_ylabel("集群总写入量 (GB)")
    ax2.set_xlabel("节点容量标准差")

    # 添加图例
    ax2.legend(loc="upper right")

    # 显示图形
    plt.show()


if __name__ == '__main__':
    draw()