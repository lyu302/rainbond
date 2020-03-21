// RAINBOND, Application Management Platform
// Copyright (C) 2014-2017 Goodrain Co., Ltd.

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package conver

import (
	"fmt"
	"strconv"

	"github.com/Sirupsen/logrus"
	"github.com/pquerna/ffjson/ffjson"

	v2 "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	route "github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	"github.com/envoyproxy/go-control-plane/pkg/cache"
	api_model "github.com/goodrain/rainbond/api/model"
	envoyv2 "github.com/goodrain/rainbond/node/core/envoy/v2"
	corev1 "k8s.io/api/core/v1"
)

//GetPluginConfigs get plugin config model
func GetPluginConfigs(configs *corev1.ConfigMap) (*api_model.ResourceSpec, string, error) {
	if configs == nil {
		return nil, "", fmt.Errorf("no config for mesh")
	}
	var rs api_model.ResourceSpec
	if err := ffjson.Unmarshal([]byte(configs.Data["plugin-config"]), &rs); err != nil {
		logrus.Errorf("unmashal etcd v error, %v", err)
		return nil, "", err
	}
	return &rs, configs.Labels["plugin_id"], nil
}

//OneNodeListerner conver listerner of on envoy node
func OneNodeListerner(serviceAlias, namespace string, configs *corev1.ConfigMap, services []*corev1.Service) ([]cache.Resource, error) {
	resources, _, err := GetPluginConfigs(configs)
	if err != nil {
		return nil, err
	}
	var listener []cache.Resource
	if resources.BaseServices != nil && len(resources.BaseServices) > 0 {
		for _, l := range upstreamListener(serviceAlias, namespace, resources.BaseServices, services) {
			if err := l.Validate(); err != nil {
				logrus.Errorf("listener validate failure %s", err.Error())
			} else {
				logrus.Debugf("create listener %s for service %s", l.Name, serviceAlias)
				listener = append(listener, l)
			}
		}
	}
	if resources.BasePorts != nil && len(resources.BasePorts) > 0 {
		for _, l := range downstreamListener(serviceAlias, namespace, resources.BasePorts) {
			if err := l.Validate(); err != nil {
				logrus.Errorf("listener validate failure %s", err.Error())
			} else {
				logrus.Debugf("create listener %s for service %s", l.Name, serviceAlias)
				listener = append(listener, l)
			}
		}
	}
	if len(listener) == 0 {
		logrus.Warningf("configmap name: %s; plugin-config: %s; create listener zero length", configs.Name, configs.Data["plugin-config"])
	}
	return listener, nil
}

//upstreamListener handle upstream app listener
// handle kubernetes inner service
func upstreamListener(serviceAlias, namespace string, dependsServices []*api_model.BaseService, services []*corev1.Service) (ldsL []*v2.Listener) {
	var ListennerConfig = make(map[string]*api_model.BaseService, len(dependsServices))
	for i, dService := range dependsServices {
		listennerName := fmt.Sprintf("%s_%s_%s_%d", namespace, serviceAlias, dService.DependServiceAlias, dService.Port)
		ListennerConfig[listennerName] = dependsServices[i]
	}
	var portMap = make(map[int32]int)
	var uniqRoute = make(map[string]*route.Route, len(services))
	var newVHL []route.VirtualHost
	for _, service := range services {
		inner, ok := service.Labels["service_type"]
		if !ok || inner != "inner" {
			continue
		}
		port := service.Spec.Ports[0].Port
		var ListenPort = port
		//listener real port
		if value, ok := service.Labels["origin_port"]; ok {
			origin, _ := strconv.Atoi(value)
			if origin != 0 {
				ListenPort = int32(origin)
			}
		}
		clusterName := fmt.Sprintf("%s_%s_%s_%d", namespace, serviceAlias, GetServiceAliasByService(service), port)
		listennerName := fmt.Sprintf("%s_%s_%s_%d", namespace, serviceAlias, GetServiceAliasByService(service), ListenPort)
		destService := ListennerConfig[listennerName]
		statPrefix := fmt.Sprintf("%s_%s", serviceAlias, GetServiceAliasByService(service))

		// Unique by listen port
		if _, ok := portMap[ListenPort]; !ok {
			//listener name depend listner port
			listenerName := fmt.Sprintf("%s_%s_%d", namespace, serviceAlias, ListenPort)
			var listener *v2.Listener
			protocol := service.Labels["port_protocol"]
			if domain, ok := service.Annotations["domain"]; ok && domain != "" && (protocol == "https" || protocol == "http") {
				route := envoyv2.CreateRouteWithHostRewrite(domain, clusterName, "/", nil, 0)
				if route != nil {
					pvh := envoyv2.CreateRouteVirtualHost(fmt.Sprintf("%s_%s_%s_%d", namespace, serviceAlias, GetServiceAliasByService(service), port), []string{"*"}, nil, *route)
					if pvh != nil {
						listener = envoyv2.CreateHTTPListener(fmt.Sprintf("%s_%s_http_%d", namespace, serviceAlias, port), envoyv2.DefaultLocalhostListenerAddress, fmt.Sprintf("%s_%d", serviceAlias, port), uint32(port), nil, *pvh)
					} else {
						logrus.Warnf("create route virtual host of domain listener %s failure", fmt.Sprintf("%s_%s_http_%d", namespace, serviceAlias, port))
					}
				}
			} else {
				listener = envoyv2.CreateTCPListener(listenerName, clusterName, envoyv2.DefaultLocalhostListenerAddress, statPrefix, uint32(ListenPort))
			}
			if listener != nil {
				ldsL = append(ldsL, listener)
			} else {
				logrus.Warningf("create tcp listenner %s failure", listenerName)
				continue
			}
			portMap[ListenPort] = len(ldsL) - 1
		}

		portProtocol, _ := service.Labels["port_protocol"]
		if destService != nil && destService.Protocol != "" {
			portProtocol = destService.Protocol
		}

		if portProtocol != "" {
			//TODO: support more protocol
			switch portProtocol {
			case "http", "https":
				var options envoyv2.RainbondPluginOptions
				if destService != nil {
					options = envoyv2.GetOptionValues(destService.Options)
				} else {
					logrus.Warningf("destService is nil for service %s", serviceAlias)
				}
				hashKey := options.RouteBasicHash()
				if oldroute, ok := uniqRoute[hashKey]; ok {
					oldrr := oldroute.Action.(*route.Route_Route)
					oldrrwc := oldrr.Route.ClusterSpecifier.(*route.RouteAction_WeightedClusters)
					weight := envoyv2.CheckWeightSum(oldrrwc.WeightedClusters.Clusters, options.Weight)
					oldrrwc.WeightedClusters.Clusters = append(oldrrwc.WeightedClusters.Clusters, &route.WeightedCluster_ClusterWeight{
						Name:   clusterName,
						Weight: envoyv2.ConversionUInt32(weight),
					})
				} else {
					var headerMatchers []*route.HeaderMatcher
					for _, header := range options.Headers {
						headerMatcher := envoyv2.CreateHeaderMatcher(header)
						if headerMatcher != nil {
							headerMatchers = append(headerMatchers, headerMatcher)
						}
					}
					var route *route.Route
					if domain, ok := service.Annotations["domain"]; ok && domain != "" {
						route = envoyv2.CreateRouteWithHostRewrite(domain, clusterName, options.Prefix, headerMatchers, options.Weight)
					} else {
						route = envoyv2.CreateRoute(clusterName, options.Prefix, headerMatchers, options.Weight)
					}

					if route != nil {
						pvh := envoyv2.CreateRouteVirtualHost(fmt.Sprintf("%s_%s_%s_%d", namespace, serviceAlias,
							GetServiceAliasByService(service), port), options.Domains, nil, *route)
						if pvh != nil {
							newVHL = append(newVHL, *pvh)
							uniqRoute[hashKey] = route
						}
					}
				}
			default:
				continue
			}
		}
	}
	logrus.Debugf("virtual host is : %v", newVHL)
	// create common http listener
	if len(newVHL) > 0 {
		//remove 80 tcp listener is exist
		if i, ok := portMap[80]; ok {
			ldsL = append(ldsL[:i], ldsL[i+1:]...)
		}
		statsPrefix := fmt.Sprintf("%s_80", serviceAlias)
		plds := envoyv2.CreateHTTPListener(fmt.Sprintf("%s_%s_http_80", namespace, serviceAlias), envoyv2.DefaultLocalhostListenerAddress, statsPrefix, 80, nil, newVHL...)
		if plds != nil {
			ldsL = append(ldsL, plds)
		} else {
			logrus.Warnf("create listenner %s failure", fmt.Sprintf("%s_%s_http_80", namespace, serviceAlias))
		}
	}
	return
}

//downstreamListener handle app self port listener
func downstreamListener(serviceAlias, namespace string, ports []*api_model.BasePort) (ls []*v2.Listener) {
	var portMap = make(map[int32]int, 0)
	for i := range ports {
		p := ports[i]
		port := int32(p.Port)
		clusterName := fmt.Sprintf("%s_%s_%d", namespace, serviceAlias, port)
		listenerName := clusterName
		statsPrefix := fmt.Sprintf("%s_%d", serviceAlias, port)
		if _, ok := portMap[port]; !ok {
			inboundConfig := envoyv2.GetRainbondInboundPluginOptions(p.Options)
			if p.Protocol == "http" || p.Protocol == "https" {
				var limit []*route.RateLimit
				if inboundConfig.OpenLimit {
					limit = []*route.RateLimit{
						&route.RateLimit{
							Actions: []*route.RateLimit_Action{
								&route.RateLimit_Action{
									ActionSpecifier: &route.RateLimit_Action_RemoteAddress_{
										RemoteAddress: &route.RateLimit_Action_RemoteAddress{},
									},
								},
							},
						},
					}
				}
				route := envoyv2.CreateRoute(clusterName, "/", nil, 100)
				if route == nil {
					logrus.Warning("create route cirtual route failure")
					continue
				}
				virtuals := envoyv2.CreateRouteVirtualHost(listenerName, []string{"*"}, limit, *route)
				if virtuals == nil {
					logrus.Warning("create route cirtual failure")
					continue
				}
				listener := envoyv2.CreateHTTPListener(listenerName, "0.0.0.0", statsPrefix, uint32(p.ListenPort), &envoyv2.RateLimitOptions{
					Enable:                inboundConfig.OpenLimit,
					Domain:                inboundConfig.LimitDomain,
					RateServerClusterName: envoyv2.DefaultRateLimitServerClusterName,
					Stage:                 0,
				}, *virtuals)
				if listener != nil {
					ls = append(ls, listener)
				}
			} else {
				listener := envoyv2.CreateTCPListener(listenerName, clusterName, "0.0.0.0", statsPrefix, uint32(p.ListenPort))
				if listener != nil {
					ls = append(ls, listener)
				} else {
					logrus.Warningf("create tcp listener %s failure", listenerName)
					continue
				}
			}
			portMap[port] = 1
		}
	}
	return
}

//GetServiceAliasByService get service alias from k8s service
func GetServiceAliasByService(service *corev1.Service) string {
	//v5.1 and later
	if serviceAlias, ok := service.Labels["service_alias"]; ok {
		return serviceAlias
	}
	//version before v5.1
	if serviceAlias, ok := service.Spec.Selector["name"]; ok {
		return serviceAlias
	}
	return ""
}
