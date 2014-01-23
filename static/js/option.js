var app = angular.module("option_app", []);

app.controller("option_controller", ["$http", "$log", "$scope", "$timeout", function($http, $log, $scope, $timeout){
  $scope.config = {};

  $scope.get_current_config = function() {
    $http.get("/api/config").success(function(data) {
      var obj = angular.fromJson(data);
      console.log(obj);
      $scope.config = obj;
    }).error(function(data){
      console.log(data);
    });
  };

  $scope.update = function(config) {
    var put_data = angular.toJson($scope.config);
    $log.debug(put_data);
    $http.put("/api/config", put_data)
      .success(function(){
        $log.info("ok");
      }).error(function(data){
        $log.info(data);
      });
  };

  $timeout($scope.get_current_config, 0);

}]);
