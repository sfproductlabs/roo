####################################################################################
# Roo
# Build with: sudo docker build -t roo .
####################################################################################

FROM golang:latest
EXPOSE 6299 6300 443 80

# update packages and install required ones
RUN apt update && apt upgrade -y && apt install -y --no-install-recommends \
#  golang \
#  git \
#  libssl-dev \
#  python-pip \
  dnsutils \
  jq \
  librocksdb-dev \
  ca-certificates \
  valgrind \
  build-essential \
  && apt autoclean -y \
  && apt autoremove -y \
  && rm -rf /var/lib/apt/lists/* 


####################################################################################

# ulimit increase (set in docker templats/aws ecs-task-definition too!!)
RUN bash -c 'echo "root hard nofile 16384" >> /etc/security/limits.conf' \
 && bash -c 'echo "root soft nofile 16384" >> /etc/security/limits.conf' \
 && bash -c 'echo "* hard nofile 16384" >> /etc/security/limits.conf' \
 && bash -c 'echo "* soft nofile 16384" >> /etc/security/limits.conf'

# ip/tcp tweaks, disable ipv6
RUN bash -c 'echo "net.core.somaxconn = 8192" >> /etc/sysctl.conf' \
 && bash -c 'echo "net.ipv4.tcp_max_tw_buckets = 1440000" >> /etc/sysctl.conf' \
 && bash -c 'echo "net.ipv6.conf.all.disable_ipv6 = 1" >> /etc/sysctl.conf' \ 
 && bash -c 'echo "net.ipv4.ip_local_port_range = 5000 65000" >> /etc/sysctl.conf' \
 && bash -c 'echo "net.ipv4.tcp_fin_timeout = 15" >> /etc/sysctl.conf' \
 && bash -c 'echo "net.ipv4.tcp_window_scaling = 1" >> /etc/sysctl.conf' \
 && bash -c 'echo "net.ipv4.tcp_syncookies = 1" >> /etc/sysctl.conf' \
 && bash -c 'echo "net.ipv4.tcp_max_syn_backlog = 8192" >> /etc/sysctl.conf' \
 && bash -c 'echo "fs.file-max=65536" >> /etc/sysctl.conf'

####################################################################################


WORKDIR /app/roo
ADD . /app/roo
RUN bash -c 'rm /app/roo/rood || exit 0'
RUN bash -c 'make'
# update the config if you need
RUN bash -c 'rm /app/roo/temp.config.json || exit 0'

####################################################################################
#ENV GODEBUG="netdns=cgo"
# startup command
CMD ["/usr/bin/nice", "-n", "5", "/app/roo/rood", "/app/roo/roo/config.json"] 
# Can also clean logs > /dev/null 2>&1
#sudo docker build -t roo .
#sudo docker run -p 443:443 -p 80:80 roo
