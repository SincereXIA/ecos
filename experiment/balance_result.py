import matplotlib.pyplot as plt
import numpy as np
import pandas as pd
from matplotlib import ticker

if __name__ == '__main__':
    # 1. 读取数据
    ceph_like_data = pd.read_csv('ceph-like.csv')
    ecos_data = pd.read_csv('ecos.csv')

    # 获取指定列
    y = []
    for index, col in ceph_like_data.items():
        if "Percent" in index:
            print(index)
            y.append(index)

    fig, ax = plt.subplots(2, 2, figsize=(12, 9))
    ((ax1, ax2), (ax3, ax4)) = ax
    ax3 = plt.subplot(212)
    ceph_like_data.plot(x="Total Write", y=y, ax=ax1)
    ecos_data.plot(x="Total Write", y=y, ax=ax2)
    ax3.plot(ceph_like_data["Total Write"], ceph_like_data["variance"])
    ax3.plot(ecos_data["Total Write"], ecos_data["variance"])
    ax3.set_xlabel("write data")
    ax3.set_ylabel("variance")

    font = {'weight': 'normal', 'size': 7}
    for r in ax:
        for a in r:
            a.legend(loc='upper left', prop=font)

    # 设置图例
    ax[0, 0].set_title('ceph-like')
    ax[0, 1].set_title('ECOS')

    # 设置坐标轴
    ax[0, 0].set_xlabel('write data')
    ax[0, 1].set_xlabel('write data')

    ax[0, 0].yaxis.set_major_formatter(ticker.PercentFormatter(xmax=1, decimals=1))
    ax[0, 1].yaxis.set_major_formatter(ticker.PercentFormatter(xmax=1, decimals=1))

    plt.show()

