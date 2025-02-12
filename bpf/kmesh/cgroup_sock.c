/*
 * Copyright 2023 The Kmesh Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at:
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.

 * Author: nlgwcy
 * Create: 2022-02-14
 */

#include <linux/in.h>
#include <linux/bpf.h>
#include <linux/tcp.h>
#include "bpf_log.h"
#include "listener.h"
#include "listener/listener.pb-c.h"
#if KMESH_ENABLE_IPV4
#if KMESH_ENABLE_HTTP

static const char kmesh_module_name[] = "kmesh_defer";
#ifdef DECLARE_VAR_ADDRESS
#undef DECLARE_VAR_ADDRESS
#define DECLARE_VAR_ADDRESS(ctx, name) \
	address_t name = {0}; \
	name.ipv4 = (ctx)->user_ip4; \
	name.port = (ctx)->user_port; \
	name.protocol = ((ctx)->protocol == IPPROTO_TCP) ?	\
	CORE__SOCKET_ADDRESS__PROTOCOL__TCP: CORE__SOCKET_ADDRESS__PROTOCOL__UDP
#endif

static inline int sock4_traffic_control(struct bpf_sock_addr *ctx)
{
	int ret;

	Listener__Listener *listener = NULL;

	DECLARE_VAR_ADDRESS(ctx, address);

	listener = map_lookup_listener(&address);
	if (listener == NULL) {
		address.ipv4 = 0;
		listener = map_lookup_listener(&address);
		if (!listener)
			return -ENOENT;
	}

#if KMESH_ENABLE_HTTP
	// defer conn
	ret = bpf_setsockopt(ctx, IPPROTO_TCP, TCP_ULP, (void *)kmesh_module_name, sizeof(kmesh_module_name));
	if (ret)
		BPF_LOG(ERR, KMESH, "bpf set sockopt failed! ret:%d\n", ret);
#else // KMESH_ENABLE_HTTP
	ret = l4_listener_manager(ctx, lisdemotener);
	if (ret != 0) {
		BPF_LOG(ERR, KMESH, "listener_manager failed, ret %d\n", ret);
		return ret;
	}
#endif // KMESH_ENABLE_HTTP

	return 0;
}

SEC("cgroup/connect4")
int cgroup_connect4_prog(struct bpf_sock_addr *ctx)
{
	int ret = sock4_traffic_control(ctx);
	return CGROUP_SOCK_OK;
}

#endif // KMESH_ENABLE_TCP
#endif // KMESH_ENABLE_IPV4

char _license[] SEC("license") = "GPL";
int _version SEC("version") = 1;
