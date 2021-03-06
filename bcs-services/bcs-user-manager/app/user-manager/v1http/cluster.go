/*
 * Tencent is pleased to support the open source community by making Blueking Container Service available.
 * Copyright (C) 2019 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package v1http

import (
	"fmt"
	"time"

	"bk-bcs/bcs-common/common"
	"bk-bcs/bcs-common/common/blog"
	"bk-bcs/bcs-services/bcs-user-manager/app/metrics"
	"bk-bcs/bcs-services/bcs-user-manager/app/user-manager/models"
	"bk-bcs/bcs-services/bcs-user-manager/app/user-manager/storages/sqlstore"
	"bk-bcs/bcs-services/bcs-user-manager/app/user-manager/utils"
	"github.com/emicklei/go-restful"
)

const (
	BcsK8sCluster = iota + 1
	BcsMesosCluster
	BcsTkeCluster
)

// CreateClusterForm
type CreateClusterForm struct {
	ClusterID        string `json:"cluster_id" validate:"required"`
	ClusterType      string `json:"cluster_type" validate:"required"`
	TkeClusterID     string `json:"tke_cluster_id"`
	TkeClusterRegion string `json:"tke_cluster_region"`
}

func CreateCluster(request *restful.Request, response *restful.Response) {
	start := time.Now()

	form := CreateClusterForm{}
	_ = request.ReadEntity(&form)

	err := utils.Validate.Struct(&form)
	if err != nil {
		metrics.RequestErrorCount.WithLabelValues("cluster", request.Request.Method).Inc()
		metrics.RequestErrorLatency.WithLabelValues("cluster", request.Request.Method).Observe(time.Since(start).Seconds())
		_ = response.WriteHeaderAndEntity(400, utils.FormatValidationError(err))
		return
	}

	user := GetUser(request)
	cluster := &models.BcsCluster{
		ID:        form.ClusterID,
		CreatorId: user.ID,
	}
	switch form.ClusterType {
	case "k8s":
		cluster.ClusterType = BcsK8sCluster
	case "mesos":
		cluster.ClusterType = BcsMesosCluster
	case "tke":
		cluster.ClusterType = BcsTkeCluster
		if form.TkeClusterID == "" || form.TkeClusterRegion == "" {
			metrics.RequestErrorCount.WithLabelValues("cluster", request.Request.Method).Inc()
			metrics.RequestErrorLatency.WithLabelValues("cluster", request.Request.Method).Observe(time.Since(start).Seconds())
			blog.Warnf("create tke cluster failed, empty tke clusterid or region")
			message := fmt.Sprintf("errcode: %d, create tke cluster failed, empty tke clusterid or region", common.BcsErrApiBadRequest)
			utils.WriteClientError(response, common.BcsErrApiBadRequest, message)
			return
		}
		cluster.TkeClusterId = form.TkeClusterID
		cluster.TkeClusterRegion = form.TkeClusterRegion
	default:
		metrics.RequestErrorCount.WithLabelValues("cluster", request.Request.Method).Inc()
		metrics.RequestErrorLatency.WithLabelValues("cluster", request.Request.Method).Observe(time.Since(start).Seconds())
		blog.Warnf("create failed, cluster type invalid")
		message := fmt.Sprintf("errcode: %d, create failed, cluster type invalid", common.BcsErrApiBadRequest)
		utils.WriteClientError(response, common.BcsErrApiBadRequest, message)
		return
	}

	clusterInDb := sqlstore.GetCluster(cluster.ID)
	if clusterInDb != nil {
		metrics.RequestErrorCount.WithLabelValues("cluster", request.Request.Method).Inc()
		metrics.RequestErrorLatency.WithLabelValues("cluster", request.Request.Method).Observe(time.Since(start).Seconds())
		blog.Warnf("create cluster failed, cluster [%s] already exist", cluster.ID)
		message := fmt.Sprintf("errcode: %d, create cluster failed, cluster [%s] already exist", common.BcsErrApiBadRequest, cluster.ID)
		utils.WriteClientError(response, common.BcsErrApiBadRequest, message)
		return
	}

	err = sqlstore.CreateCluster(cluster)
	if err != nil {
		metrics.RequestErrorCount.WithLabelValues("cluster", request.Request.Method).Inc()
		metrics.RequestErrorLatency.WithLabelValues("cluster", request.Request.Method).Observe(time.Since(start).Seconds())
		blog.Errorf("failed to create cluster [%s]: %s", cluster.ID, err.Error())
		message := fmt.Sprintf("errcode: %d, create cluster [%s] failed, error: %s", common.BcsErrApiInternalDbError, cluster.ID, err.Error())
		utils.WriteServerError(response, common.BcsErrApiInternalDbError, message)
		return
	}

	data := utils.CreateResponeData(nil, "success", *cluster)
	_, _ = response.Write([]byte(data))

	metrics.RequestCount.WithLabelValues("cluster", request.Request.Method).Inc()
	metrics.RequestLatency.WithLabelValues("cluster", request.Request.Method).Observe(time.Since(start).Seconds())
}
