import matplotlib.pyplot as plt
import numpy as np
import pandas as pd
from matplotlib import ticker

from matplotlib.font_manager import FontManager
import subprocess
from matplotlib.font_manager import FontProperties
import numpy as np

font_path = "/usr/local/share/fonts/SourceHanSerifSC-Regular.otf"
# 加载中文字体
font = FontProperties(fname=font_path, size=11)
# set font by font_path
plt.rcParams['font.sans-serif'] = ['SimHei']  # 选择中文字体
plt.rcParams['font.family'] = ['Source Han Serif SC']  # 选择中文字体
# 设置字号
plt.rcParams['font.size'] = 13

if __name__ == '__main__':
    # 1. 读取数据
    ceph_like_data = pd.read_csv('0.csv')
    ecos_data = pd.read_csv('1.csv')

    # 获取指定列
    y = []
    rename_dict = {}
    for index, col in ceph_like_data.items():
        if "Percent" in index:
            print(index)
            rename_dict[index] = index.replace(" Used Percent", "")
            # 重命名，去除 Used Percent
            index = index.replace(" Used Percent", "")
            y.append(index)

    # x 轴单位为 byte，转换为 GB
    ceph_like_data["Total Write"] = ceph_like_data["Total Write"] / 1024 / 1024 / 1024
    ecos_data["Total Write"] = ecos_data["Total Write"] / 1024 / 1024 / 1024

    # 修改列名
    ceph_like_data.rename(columns=rename_dict, inplace=True)
    ecos_data.rename(columns=rename_dict, inplace=True)
    fig, ax = plt.subplots(2, 2, figsize=(12, 9))
    ((ax1, ax2), (ax3, ax4)) = ax
    ax3 = plt.subplot(212)
    ceph_like_data.plot(x="Total Write", y=y, ax=ax1)
    ecos_data.plot(x="Total Write", y=y, ax=ax2)
    ax1.set_xlabel("总写入量（GB）")
    ax1.set_ylabel("节点存储使用率")
    ax2.set_xlabel("总写入量（GB） ")
    ax2.set_ylabel("节点存储使用率")


    # 设置间距
    plt.subplots_adjust(wspace=0.3, hspace=0.3)

    # y 轴百分比
    ax1.yaxis.set_major_formatter(ticker.PercentFormatter(xmax=1, decimals=0))
    ax2.yaxis.set_major_formatter(ticker.PercentFormatter(xmax=1, decimals=0))

    # 绘制 Ceph-like variance 和 ECOS variance 在 ax3 中
    # 重命名列名
    ceph_like_data.rename(columns={"variance": "Ceph-like"}, inplace=True)
    ecos_data.rename(columns={"variance": "ECOS"}, inplace=True)

    # 3. 绘制
    ceph_like_data.plot(x="Total Write", y="Ceph-like", ax=ax3)
    ecos_data.plot(x="Total Write", y="ECOS", ax=ax3)


    # 标题
    ax3.set_xlabel("总写入量（GB）")
    ax3.set_ylabel("节点存储使用率标准差")
    # 添加图例
    ax1.legend(loc='upper left', prop=font)
    ax2.legend(loc='upper left', prop=font)
    ax3.legend(loc='upper left', prop=font)

    for r in ax:
        for a in r:
            a.legend(loc='upper left', prop=font)

    # 设置图例
    ax[0, 0].set_title('放置方案: Ceph-like')
    ax[0, 1].set_title('放置方案: ECOS')


    plt.show()

