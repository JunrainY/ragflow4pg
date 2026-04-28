## 打包
### 官网文档 https://ragflow.io/docs/dev/build_docker_image
### 参考 https://blog.csdn.net/tizaitian/article/details/150868815

cd ragflow/
uv pip install huggingface_hub
source .venv/bin/activate
python download_deps.py --china-mirrors
docker build -f Dockerfile.deps -t infiniflow/ragflow_deps .
docker build -f Dockerfile -t infiniflow/ragflow:nightly --build-arg NEED_MIRROR=1 .


## 启动
docker compose -f docker/docker-compose-macos.yml up -d
### 测试
docker logs -f docker-ragflow-cpu-1
### 访问 账号密码： leon07@qq.com:123456qq
http://localhost  

### 官方文档
https://ragflow.io/docs/dev/


## 修改代码
修改完毕后，保存文件并重启 Docker 容器，以使新的环境变量生效：

docker compose -f docker/docker-compose.yml up -d --force-recreate